package apikey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/constant"
	"github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

// ── Моки ────────────────────────────────────────────────

type svcMock struct {
	byHash  map[string]*model.Main
	byId    map[string]*model.Main
	created []*model.Edit
	updated []*model.Edit
	touched []string
}

func (m *svcMock) List(_ context.Context, _ *model.ListReq) ([]*model.Main, int64, error) {
	return nil, 0, nil
}

func (m *svcMock) Get(_ context.Context, id string, errNE bool) (*model.Main, bool, error) {
	item, ok := m.byId[id]
	if !ok && errNE {
		return nil, false, errs.ObjectNotFound
	}
	return item, ok, nil
}

func (m *svcMock) GetByKeyHash(_ context.Context, keyHash string) (*model.Main, bool, error) {
	item, ok := m.byHash[keyHash]
	return item, ok, nil
}

func (m *svcMock) Create(_ context.Context, obj *model.Edit) (string, error) {
	m.created = append(m.created, obj)
	if m.byId == nil {
		m.byId = map[string]*model.Main{}
	}
	created := &model.Main{Id: "new-id"}
	if obj.UsrId != nil {
		created.UsrId = *obj.UsrId
	}
	if obj.Active != nil {
		created.Active = *obj.Active
	}
	if obj.Scope != nil {
		created.Scope = *obj.Scope
	}
	if obj.Name != nil {
		created.Name = *obj.Name
	}
	m.byId["new-id"] = created
	return "new-id", nil
}

func (m *svcMock) Update(_ context.Context, _ string, obj *model.Edit) error {
	m.updated = append(m.updated, obj)
	return nil
}

func (m *svcMock) TouchLastUsed(_ context.Context, id string) error {
	m.touched = append(m.touched, id)
	return nil
}

func (m *svcMock) Delete(_ context.Context, _ string) error { return nil }

type usrSvcMock struct {
	usrs map[int64]*usrModel.Main
}

func (m *usrSvcMock) Get(_ context.Context, id int64, errNE bool) (*usrModel.Main, bool, error) {
	usr, ok := m.usrs[id]
	if !ok && errNE {
		return nil, false, errs.ObjectNotFound
	}
	return usr, ok, nil
}

type sessionSvcMock struct {
	session *sessionModel.Session
}

func (m *sessionSvcMock) FromContext(_ context.Context) *sessionModel.Session { return m.session }
func (m *sessionSvcMock) CtxIsAuthorized(_ context.Context) bool              { return m.session.IsAuthorized() }
func (m *sessionSvcMock) CtxIsAdmin(_ context.Context) bool                   { return m.session.IsAdmin() }

// txmStub выполняет функцию транзакции без реальной транзакции.
type txmStub struct{}

func (txmStub) TxFn(ctx context.Context, f func(context.Context) error) error {
	return f(ctx)
}

// auditRecStub — no-op регистратор аудита.
// passwordStub принимает только testPassword (как usr usecase — текущий
// пароль вошедшего пользователя).
type passwordStub struct{}

const testPassword = "current-pw"

func (passwordStub) ConfirmPassword(_ context.Context, currentPassword string) error {
	if currentPassword != testPassword {
		return errs.CurrentPasswordInvalid
	}
	return nil
}

type auditRecStub struct{}

func (auditRecStub) RecordApiKey(context.Context, *model.Main, *model.Main, *string) error {
	return nil
}

// ── Тесты ───────────────────────────────────────────────

func TestSessionFromKey(t *testing.T) {
	t.Parallel()

	activeKey, activeHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	inactiveKey, inactiveHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	orphanKey, orphanHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	mcpOnlyKey, mcpOnlyHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	svc := &svcMock{byHash: map[string]*model.Main{
		activeHash:   {Id: "k1", UsrId: 10, Active: true},
		inactiveHash: {Id: "k2", UsrId: 10, Active: false},
		orphanHash:   {Id: "k3", UsrId: 66, Active: true},
		mcpOnlyHash:  {Id: "k4", UsrId: 10, Active: true, Scope: constant.ApiKeyScopeMcpOnly},
	}}
	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true, IsAdmin: false, AppIds: []string{"app1"}},
	}}

	u := New(svc, usrSvc, &sessionSvcMock{}, txmStub{}, auditRecStub{}, passwordStub{})

	// валидный ключ активного пользователя
	session, err := u.SessionFromKey(context.Background(), activeKey)
	require.NoError(t, err)
	assert.Equal(t, int64(10), session.Id)
	assert.False(t, session.Admin)
	assert.Equal(t, []string{"app1"}, session.AppIds)
	assert.Equal(t, []string{"k1"}, svc.touched)

	// повторное использование в пределах минуты БД не трогает
	_, err = u.SessionFromKey(context.Background(), activeKey)
	require.NoError(t, err)
	assert.Len(t, svc.touched, 1)

	// неактивный ключ
	_, err = u.SessionFromKey(context.Background(), inactiveKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// владелец не найден
	_, err = u.SessionFromKey(context.Background(), orphanKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// не-ключ (JWT и мусор)
	_, err = u.SessionFromKey(context.Background(), "eyJhbGciOi...")
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// валидный по формату, но неизвестный ключ
	unknownKey, _, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	_, err = u.SessionFromKey(context.Background(), unknownKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// mcp_only-ключ: основной API отвергает, MCP-эндпоинт принимает
	_, err = u.SessionFromKey(context.Background(), mcpOnlyKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	session, err = u.McpSessionFromKey(context.Background(), mcpOnlyKey)
	require.NoError(t, err)
	assert.Equal(t, int64(10), session.Id)

	// обычный ключ MCP-эндпоинт тоже принимает
	_, err = u.McpSessionFromKey(context.Background(), activeKey)
	require.NoError(t, err)
}

func TestCreate_Permissions(t *testing.T) {
	t.Parallel()

	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true},
		20: {Id: 20, Active: true},
	}}

	// не-админ создаёт ключ себе
	svc := &svcMock{}
	u := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 10}}, txmStub{}, auditRecStub{}, passwordStub{})

	id, key, err := u.Create(context.Background(), "мой ключ", nil, constant.ApiKeyScopeMcpOnly, testPassword)
	require.NoError(t, err)
	assert.Equal(t, "new-id", id)
	assert.NotEmpty(t, key)
	require.Len(t, svc.created, 1)
	assert.Equal(t, int64(10), *svc.created[0].UsrId)
	assert.Equal(t, constant.ApiKeyScopeMcpOnly, *svc.created[0].Scope)
	// в БД уходит хэш, не сам ключ
	assert.Equal(t, apikeyService.HashKey(key), *svc.created[0].KeyHash)

	// не-админ не может выпустить ключ другому пользователю
	_, _, err = u.Create(context.Background(), "чужой", new(int64(20)), constant.ApiKeyScopeMcpOnly, testPassword)
	assert.ErrorIs(t, err, errs.NoPermission)

	// не-админ не может выпустить ключ с полным доступом к API
	_, _, err = u.Create(context.Background(), "полный", nil, constant.ApiKeyScopeFull, testPassword)
	assert.ErrorIs(t, err, errs.NoPermission)

	// админ — может
	uAdmin := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 1, Admin: true}}, txmStub{}, auditRecStub{}, passwordStub{})
	_, _, err = uAdmin.Create(context.Background(), "для сервисного", new(int64(20)), constant.ApiKeyScopeFull, testPassword)
	require.NoError(t, err)
	assert.Equal(t, int64(20), *svc.created[len(svc.created)-1].UsrId)
	assert.Equal(t, constant.ApiKeyScopeFull, *svc.created[len(svc.created)-1].Scope)

	// без верного текущего пароля ключ не выпускается: иначе украденная
	// сессия превращалась бы в бессрочный доступ
	before := len(svc.created)
	for _, pw := range []string{"", "wrong"} {
		_, _, err = u.Create(context.Background(), "без пароля", nil, constant.ApiKeyScopeMcpOnly, pw)
		assert.ErrorIs(t, err, errs.CurrentPasswordInvalid)
		_, _, err = uAdmin.Create(context.Background(), "без пароля", new(int64(20)), constant.ApiKeyScopeFull, pw)
		assert.ErrorIs(t, err, errs.CurrentPasswordInvalid)
	}
	assert.Len(t, svc.created, before)

	// неавторизованный
	uAnon := New(svc, usrSvc, &sessionSvcMock{}, txmStub{}, auditRecStub{}, passwordStub{})
	_, _, err = uAnon.Create(context.Background(), "x", nil, constant.ApiKeyScopeFull, testPassword)
	assert.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUpdate_ScopePermissions(t *testing.T) {
	t.Parallel()

	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true},
	}}

	newSvc := func() *svcMock {
		return &svcMock{byId: map[string]*model.Main{
			"k-mcp":  {Id: "k-mcp", UsrId: 10, Active: true, Scope: constant.ApiKeyScopeMcpOnly},
			"k-full": {Id: "k-full", UsrId: 10, Active: true, Scope: constant.ApiKeyScopeFull},
			"k-ro":   {Id: "k-ro", UsrId: 10, Active: true, Scope: constant.ApiKeyScopeReadOnly},
		}}
	}

	// не-админ не может расширить свой mcp_only-ключ до full
	svc := newSvc()
	u := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 10}}, txmStub{}, auditRecStub{}, passwordStub{})
	err := u.Update(context.Background(), "k-mcp", nil, nil, new(constant.ApiKeyScopeFull))
	assert.ErrorIs(t, err, errs.NoPermission)
	assert.Empty(t, svc.updated)

	// и не может «переключить» read_only на mcp_only (не сужение full)
	err = u.Update(context.Background(), "k-ro", nil, nil, new(constant.ApiKeyScopeMcpOnly))
	assert.ErrorIs(t, err, errs.NoPermission)

	// но может переименовать свой full-ключ, даже присылая scope как есть
	err = u.Update(context.Background(), "k-full", nil, new("новое имя"), new(constant.ApiKeyScopeFull))
	require.NoError(t, err)
	require.Len(t, svc.updated, 1)

	// и может сузить свой full-ключ до mcp_only или read_only
	err = u.Update(context.Background(), "k-full", nil, nil, new(constant.ApiKeyScopeMcpOnly))
	require.NoError(t, err)
	err = u.Update(context.Background(), "k-full", nil, nil, new(constant.ApiKeyScopeReadOnly))
	require.NoError(t, err)

	// неизвестный scope отвергается
	err = u.Update(context.Background(), "k-full", nil, nil, new("root"))
	assert.ErrorIs(t, err, errs.InvalidRequest)

	// админ меняет scope свободно
	svcAdmin := newSvc()
	uAdmin := New(svcAdmin, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 1, Admin: true}}, txmStub{}, auditRecStub{}, passwordStub{})
	err = uAdmin.Update(context.Background(), "k-mcp", nil, nil, new(constant.ApiKeyScopeFull))
	require.NoError(t, err)
	require.Len(t, svcAdmin.updated, 1)
}

// Сессия по API-ключу или OAuth-подключению ключами не управляет: утёкший
// ключ не выпускает себе новые, не меняет scope и не удаляет ключи. Читать
// список ключей может.
func TestKeySession_CannotManageKeys(t *testing.T) {
	t.Parallel()

	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		1: {Id: 1, Active: true, IsAdmin: true},
	}}
	svc := &svcMock{byId: map[string]*model.Main{
		"k-own": {Id: "k-own", UsrId: 1, Active: true, Scope: constant.ApiKeyScopeFull},
	}}

	// ключ админа: права владельца, но это не вход в админку
	keySession := &sessionModel.Session{
		Id: 1, Admin: true, Source: constant.SourceApi, ApiKeyId: "k-own", ApiKeyName: "ci",
	}
	u := New(svc, usrSvc, &sessionSvcMock{session: keySession}, txmStub{}, auditRecStub{}, passwordStub{})

	for _, scope := range []string{constant.ApiKeyScopeFull, constant.ApiKeyScopeReadOnly, constant.ApiKeyScopeMcpOnly} {
		_, _, err := u.Create(context.Background(), "new", nil, scope, testPassword)
		assert.ErrorIs(t, err, errs.NoPermission, scope)
	}
	assert.Empty(t, svc.created)

	err := u.Update(context.Background(), "k-own", new(false), nil, nil)
	assert.ErrorIs(t, err, errs.NoPermission)
	assert.Empty(t, svc.updated)

	err = u.Delete(context.Background(), "k-own")
	assert.ErrorIs(t, err, errs.NoPermission)

	// чтение оставлено
	listReq := &model.ListReq{}
	listReq.PageSize = 10
	_, _, err = u.List(context.Background(), listReq)
	require.NoError(t, err)

	// тот же пользователь из входа в админку ключами управляет
	uHuman := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 1, Admin: true}}, txmStub{}, auditRecStub{}, passwordStub{})
	_, _, err = uHuman.Create(context.Background(), "new", nil, constant.ApiKeyScopeFull, testPassword)
	require.NoError(t, err)
	require.NoError(t, uHuman.Update(context.Background(), "k-own", new(false), nil, nil))
	require.NoError(t, uHuman.Delete(context.Background(), "k-own"))
}

// Сессия по ключу наследует доступ владельца к приложениям, включая all_apps;
// владелец без приложений и без all_apps не получает ничего.
func TestSessionFromKey_AppAccess(t *testing.T) {
	t.Parallel()

	allKey, allHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	noneKey, noneHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	svc := &svcMock{byHash: map[string]*model.Main{
		allHash:  {Id: "k-all", UsrId: 10, Active: true},
		noneHash: {Id: "k-none", UsrId: 11, Active: true},
	}}
	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true, AllApps: true},
		11: {Id: 11, Active: true, AppIds: []string{}},
	}}
	u := New(svc, usrSvc, &sessionSvcMock{}, txmStub{}, auditRecStub{}, passwordStub{})

	session, err := u.SessionFromKey(context.Background(), allKey)
	require.NoError(t, err)
	assert.True(t, session.AllApps)
	assert.True(t, session.HasAppAccess("any-app"))

	session, err = u.SessionFromKey(context.Background(), noneKey)
	require.NoError(t, err)
	assert.False(t, session.AllApps)
	assert.False(t, session.HasAppAccess("any-app"))
	ids, all := session.AccessibleAppIds()
	assert.False(t, all)
	assert.Empty(t, ids)
}
