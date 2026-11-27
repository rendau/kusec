package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/config"
	"github.com/rendau/kusec/internal/constant"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/util"
)

const (
	testHashKey = "test-hash-key"

	oldConfigValue = "postgres://old-host:5432/billing"
	newConfigValue = "postgres://new-host:5432/billing"
)

// auditSvcMock отдаёт свежую копию записей на каждый вызов: usecase правит
// их на месте. Условия доступа применяет так же, как репозиторий
// (getConditions в internal/domain/audit/repo/db).
type auditSvcMock struct {
	build func() []*auditModel.Main
}

func (m *auditSvcMock) List(_ context.Context, pars *auditModel.ListReq) ([]*auditModel.Main, int64, error) {
	items := lo.Filter(m.build(), func(item *auditModel.Main, _ int) bool {
		switch {
		case len(pars.AppIds) > 0:
			if item.AppId == nil || !lo.Contains(pars.AppIds, *item.AppId) {
				return false
			}
		case pars.AppId != nil:
			if item.AppId == nil || *item.AppId != *pars.AppId {
				return false
			}
		}
		if pars.OnlyWithApp && item.AppId == nil {
			return false
		}
		if pars.EntityType != nil && item.EntityType != *pars.EntityType {
			return false
		}
		return true
	})
	return items, int64(len(items)), nil
}

// auditFixture — записи, как их пишет регистратор аудита
// (internal/service/audit/changes.go).
func auditFixture() []*auditModel.Main {
	longHash := util.ValueFingerprint(testHashKey, strings.Repeat("x", 5000))

	return []*auditModel.Main{
		// config_item: update — old/new целиком
		{
			Id: 1, EntityType: auditModel.EntityConfigItem, EntityId: "ci1", Key: "PG_DSN", Action: auditModel.ActionUpdate,
			Changes: []auditModel.Change{
				{Field: "description", Old: new("old desc"), New: new("new desc")},
				{Field: "value", Old: new(oldConfigValue), New: new(newConfigValue)},
			},
		},
		// config_item: create — только new
		{
			Id: 2, EntityType: auditModel.EntityConfigItem, EntityId: "ci2", Key: "HOST", Action: auditModel.ActionCreate,
			Changes: []auditModel.Change{
				{Field: "key", New: new("HOST")},
				{Field: "value", New: new(newConfigValue)},
			},
		},
		// config_item: delete — только old
		{
			Id: 3, EntityType: auditModel.EntityConfigItem, EntityId: "ci3", Key: "HOST", Action: auditModel.ActionDelete,
			Changes: []auditModel.Change{
				{Field: "value", Old: new(oldConfigValue)},
			},
		},
		// config_item: значение длиннее лимита — уже отпечаток и размер
		{
			Id: 4, EntityType: auditModel.EntityConfigItem, EntityId: "ci4", Key: "BIG", Action: auditModel.ActionCreate,
			Changes: []auditModel.Change{
				{Field: "value", NewHash: longHash, NewSize: new(int64(5000)), Truncated: true},
			},
		},
		// item секрета — отпечаток и размер с самого начала
		{
			Id: 5, EntityType: auditModel.EntityItem, EntityId: "i1", Key: "PG_PASSWORD", Action: auditModel.ActionUpdate,
			Changes: []auditModel.Change{
				{
					Field:   "value",
					OldHash: util.ValueFingerprint(testHashKey, "old-secret"), OldSize: new(int64(10)),
					NewHash: util.ValueFingerprint(testHashKey, "new-secret"), NewSize: new(int64(10)),
				},
			},
		},
		// другая сущность: поля не трогаются
		{
			Id: 6, EntityType: auditModel.EntityConfigMap, EntityId: "cm1", Action: auditModel.ActionUpdate,
			Changes: []auditModel.Change{
				{Field: "description", Old: new("a"), New: new("b")},
			},
		},
	}
}

func listAudit(t *testing.T, session *sessionModel.Session) []*auditModel.Main {
	t.Helper()

	sessionSvc := sessionService.New("test-secret")
	usecase := New(&auditSvcMock{build: auditFixture}, sessionSvc)

	items, tCount, err := usecase.List(
		sessionSvc.WithContext(context.Background(), session),
		&auditModel.ListReq{ListParams: commonModel.ListParams{PageSize: 100}},
	)
	require.NoError(t, err)
	require.Equal(t, int64(len(items)), tCount)

	return items
}

func useTestHashKey(t *testing.T) {
	t.Helper()

	prev := config.Conf.AuditHashKey
	config.Conf.AuditHashKey = testHashKey
	t.Cleanup(func() { config.Conf.AuditHashKey = prev })
}

// Ключу «только чтение» значения config_item не показываются в ConfigItem/List —
// не должны показываться и через аудит: вместо них размер и отпечаток, как у
// значений секретов.
func TestList_ReadOnlyHidesConfigValues(t *testing.T) {
	useTestHashKey(t)

	items := listAudit(t, &sessionModel.Session{
		Id: 1, Admin: true, Scope: constant.ApiKeyScopeReadOnly, Source: constant.SourceApi,
	})
	require.Len(t, items, 6)

	// значений нет нигде в ответе
	raw, err := json.Marshal(items)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), oldConfigValue)
	assert.NotContains(t, string(raw), newConfigValue)
	assert.NotContains(t, string(raw), "old-host")
	assert.NotContains(t, string(raw), "new-host")

	oldHash := util.ValueFingerprint(testHashKey, oldConfigValue)
	newHash := util.ValueFingerprint(testHashKey, newConfigValue)
	size := int64(len(oldConfigValue))

	// update: обе стороны — отпечаток и размер; соседнее поле не тронуто
	assert.Equal(t, []auditModel.Change{
		{Field: "description", Old: new("old desc"), New: new("new desc")},
		{Field: "value", OldHash: oldHash, OldSize: &size, NewHash: newHash, NewSize: &size},
	}, items[0].Changes)

	// create / delete: только одна сторона
	assert.Equal(t, []auditModel.Change{
		{Field: "key", New: new("HOST")},
		{Field: "value", NewHash: newHash, NewSize: &size},
	}, items[1].Changes)
	assert.Equal(t, []auditModel.Change{
		{Field: "value", OldHash: oldHash, OldSize: &size},
	}, items[2].Changes)

	// уже сведённые к отпечатку записи и чужие сущности — без изменений
	fixture := auditFixture()
	for _, i := range []int{3, 4, 5} {
		assert.Equal(t, fixture[i].Changes, items[i].Changes, "запись %d", fixture[i].Id)
	}

	// формат тот же, что у значения секрета: отпечаток + размер, без old/new
	secretChange := items[4].Changes[0]
	configChange := items[0].Changes[1]
	assert.Nil(t, configChange.Old)
	assert.Nil(t, configChange.New)
	assert.Len(t, configChange.NewHash, len(secretChange.NewHash))
	assert.Equal(t, secretChange.Truncated, configChange.Truncated)
}

// Сессиям, которым значения конфигмапов видны (вход в админку, full-ключ,
// MCP), аудит отдаётся как есть.
func TestList_ConfigValuesVisibleToOthers(t *testing.T) {
	useTestHashKey(t)

	for name, session := range map[string]*sessionModel.Session{
		"вход в админку": {Id: 1, Admin: true, Source: constant.SourceUi},
		"full-ключ":      {Id: 1, Admin: true, Scope: constant.ApiKeyScopeFull, Source: constant.SourceApi},
		"mcp":            {Id: 1, Admin: true, Scope: constant.ApiKeyScopeMcpOnly, Source: constant.SourceMcp},
	} {
		t.Run(name, func(t *testing.T) {
			items := listAudit(t, session)

			fixture := auditFixture()
			require.Len(t, items, len(fixture))
			for i := range fixture {
				assert.Equal(t, fixture[i].Changes, items[i].Changes, "запись %d", fixture[i].Id)
			}
		})
	}
}

// ── Доступ к записям ────────────────────────────────────

// accessFixture — записи двух приложений и записи без приложения: учётки,
// ключи, OAuth-подключения, общий sync.
func accessFixture() []*auditModel.Main {
	return []*auditModel.Main{
		{Id: 1, EntityType: auditModel.EntityItem, EntityId: "i1", AppId: new("app1"), Action: auditModel.ActionUpdate},
		{Id: 2, EntityType: auditModel.EntityConfigItem, EntityId: "ci1", AppId: new("app2"), Action: auditModel.ActionCreate},
		{Id: 3, EntityType: auditModel.EntitySyncRun, EntityId: "run1", AppId: new("app1"), Action: auditModel.ActionSync},
		{
			Id: 4, EntityType: auditModel.EntityUsr, EntityId: "5", Action: auditModel.ActionCreate,
			Changes: []auditModel.Change{
				{Field: "username", New: new("root")},
				{Field: "is_admin", New: new("true")},
			},
		},
		{
			Id: 5, EntityType: auditModel.EntityApiKey, EntityId: "k1", Action: auditModel.ActionCreate,
			Changes: []auditModel.Change{{Field: "name", New: new("ci-deploy")}},
		},
		{Id: 6, EntityType: auditModel.EntityOAuthGrant, EntityId: "g1", Action: auditModel.ActionCreate},
		{Id: 7, EntityType: auditModel.EntitySyncRun, EntityId: "run2", Action: auditModel.ActionSync}, // общий sync
	}
}

func listAccess(t *testing.T, session *sessionModel.Session, pars *auditModel.ListReq) ([]int64, error) {
	t.Helper()

	sessionSvc := sessionService.New("test-secret")
	usecase := New(&auditSvcMock{build: accessFixture}, sessionSvc)

	pars.ListParams = commonModel.ListParams{PageSize: 100}
	items, tCount, err := usecase.List(sessionSvc.WithContext(context.Background(), session), pars)
	if err != nil {
		return nil, err
	}
	require.Equal(t, int64(len(items)), tCount)

	return lo.Map(items, func(item *auditModel.Main, _ int) int64 { return item.Id }), nil
}

// Записи без приложения (учётки, API-ключи, OAuth-подключения, общий sync)
// видит только админ. Не-админ с all_apps видит записи всех приложений, но
// не весь аудит: иначе ему — а с ним его read_only-ключу и MCP-инструменту
// audit_list — открывались бы логины, флаги админа и имена ключей.
func TestList_NonAdminSeesOnlyAppRecords(t *testing.T) {
	useTestHashKey(t)

	all := []int64{1, 2, 3, 4, 5, 6, 7}
	apps := []int64{1, 2, 3}

	cases := []struct {
		name    string
		session *sessionModel.Session
		want    []int64
	}{
		{"админ, вход в админку", &sessionModel.Session{Id: 1, Admin: true, Source: constant.SourceUi}, all},
		{
			"админ, read_only-ключ",
			&sessionModel.Session{Id: 1, Admin: true, Scope: constant.ApiKeyScopeReadOnly, Source: constant.SourceApi, ApiKeyId: "k"},
			all,
		},
		{"не-админ с all_apps, вход в админку", &sessionModel.Session{Id: 2, AllApps: true, Source: constant.SourceUi}, apps},
		{
			"не-админ с all_apps, read_only-ключ",
			&sessionModel.Session{Id: 2, AllApps: true, Scope: constant.ApiKeyScopeReadOnly, Source: constant.SourceApi, ApiKeyId: "k"},
			apps,
		},
		{
			"не-админ с all_apps, MCP (audit_list)",
			&sessionModel.Session{Id: 2, AllApps: true, Scope: constant.ApiKeyScopeMcpOnly, Source: constant.SourceMcp, ApiKeyId: "k"},
			apps,
		},
		{
			"не-админ с all_apps и списком приложений",
			&sessionModel.Session{Id: 2, AllApps: true, AppIds: []string{"app1"}, Source: constant.SourceUi},
			apps,
		},
		{"не-админ со списком приложений", &sessionModel.Session{Id: 3, AppIds: []string{"app1"}, Source: constant.SourceUi}, []int64{1, 3}},
		{"не-админ без приложений", &sessionModel.Session{Id: 4, Source: constant.SourceUi}, []int64{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := listAccess(t, tc.session, &auditModel.ListReq{})
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.want, got)
		})
	}

	allApps := &sessionModel.Session{Id: 2, AllApps: true, Source: constant.SourceUi}
	listed := &sessionModel.Session{Id: 3, AppIds: []string{"app1"}, Source: constant.SourceUi}

	// фильтром по типу записи без приложения не достать
	for _, entityType := range []string{auditModel.EntityUsr, auditModel.EntityApiKey, auditModel.EntityOAuthGrant} {
		got, err := listAccess(t, allApps, &auditModel.ListReq{EntityType: new(entityType)})
		require.NoError(t, err)
		assert.Empty(t, got, entityType)
	}
	got, err := listAccess(t, allApps, &auditModel.ListReq{EntityType: new(auditModel.EntitySyncRun)})
	require.NoError(t, err)
	assert.Equal(t, []int64{3}, got, "из запусков sync — только запуск приложения, общий не виден")

	// условия доступа из запроса не принимаются: их выставляет только usecase
	got, err = listAccess(t, &sessionModel.Session{Id: 1, Admin: true}, &auditModel.ListReq{OnlyWithApp: true, AppIds: []string{"app1"}})
	require.NoError(t, err)
	assert.ElementsMatch(t, all, got)

	// фильтр по приложению: своё — да, чужое — отказ
	got, err = listAccess(t, allApps, &auditModel.ListReq{AppId: new("app1")})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 3}, got)
	got, err = listAccess(t, listed, &auditModel.ListReq{AppId: new("app1")})
	require.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 3}, got)
	_, err = listAccess(t, listed, &auditModel.ListReq{AppId: new("app2")})
	require.ErrorIs(t, err, errs.NoPermission)

	// без входа — not_authorized
	_, err = listAccess(t, nil, &auditModel.ListReq{})
	require.ErrorIs(t, err, errs.NotAuthorized)
}
