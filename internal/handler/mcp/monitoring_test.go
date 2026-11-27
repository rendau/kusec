package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	appModel "github.com/rendau/kusec/internal/domain/app/model"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	auditUsc "github.com/rendau/kusec/internal/usecase/audit"
	syncrunUsc "github.com/rendau/kusec/internal/usecase/syncrun"
)

// ── Моки журнала sync и аудита ──────────────────────────

type syncRunSvcMock struct {
	runs []*syncrunModel.Main
}

// копии: usecase фильтрует объекты запуска на месте
func cloneRun(v *syncrunModel.Main, withObjects bool) *syncrunModel.Main {
	cp := *v
	cp.Objects = nil
	if withObjects {
		for _, obj := range v.Objects {
			objCp := *obj
			cp.Objects = append(cp.Objects, &objCp)
		}
	}
	return &cp
}

func (m *syncRunSvcMock) List(context.Context, *syncrunModel.ListReq) ([]*syncrunModel.Main, int64, error) {
	result := make([]*syncrunModel.Main, 0, len(m.runs))
	for _, run := range m.runs {
		result = append(result, cloneRun(run, false))
	}
	return result, int64(len(result)), nil
}

func (m *syncRunSvcMock) Get(_ context.Context, id string, _ bool) (*syncrunModel.Main, bool, error) {
	for _, run := range m.runs {
		if run.Id == id {
			return cloneRun(run, true), true, nil
		}
	}
	return nil, false, errs.ObjectNotFound
}

type appListSvcMock struct{}

func (appListSvcMock) List(context.Context, *appModel.ListReq) ([]*appModel.Main, int64, error) {
	return []*appModel.Main{{Id: "app1", Namespace: "prod"}}, 1, nil
}

type auditListSvcMock struct {
	items []*auditModel.Main
}

func (m *auditListSvcMock) List(context.Context, *auditModel.ListReq) ([]*auditModel.Main, int64, error) {
	return m.items, int64(len(m.items)), nil
}

// Отпечатки, которые агенту видеть нельзя: содержимое целых объектов и
// значения конфигмапов. В ответах инструментов их быть не должно.
const (
	secretContentHash    = "5ec4e7c0n7en7a11"
	configMapContentHash = "c0nf19c0n7en7b22"
	configOldValueHash   = "c0nf190ldva1c333"
	configNewValueHash   = "c0nf19newva1d444"
	// отпечатки значения item-а секрета — те же, что value_hash: остаются
	itemOldValueHash = "17em01dva1ue5555"
	itemNewValueHash = "17emnewva1ue6666"
)

// TestMonitoringNoContentFingerprints — агенту не отдаются отпечатки, по
// которым ограничитель записи известных значений (literal_limit.go)
// обходится через конфигмапы.
//
// content_hash объекта в журнале sync считается одинаково для Secret и
// ConfigMap. Конфигмапы агент пишет без ограничений: конфигмап из одного
// ключа с догадкой, один sync — и сравнение content_hash с секретом из
// одного ключа проверяет догадку. То же — отпечаток значения конфигмапа
// длиннее лимита в аудите: он считается как value_hash секрета.
func TestMonitoringNoContentFingerprints(t *testing.T) {
	t.Parallel()

	key, keyHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	startedAt := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	sessionSvc := sessionService.New("test-secret")
	apikeyUsecase := apikeyUsc.New(
		&apikeySvcMock{byHash: map[string]*apikeyModel.Main{
			keyHash: {Id: "k1", UsrId: 10, Active: true, Scope: "mcp_only"},
		}},
		&usrSvcMock{usrs: map[int64]*usrModel.Main{
			10: {Id: 10, Active: true, AppIds: []string{"app1"}},
		}},
		nil,
		txmStub{},
		auditRecStub{},
		nil,
	)
	syncrunUsecase := syncrunUsc.New(
		&syncRunSvcMock{runs: []*syncrunModel.Main{{
			Id: "run1", StartedAt: startedAt, Status: syncrunModel.StatusOk, ActorName: "agent",
			Source: "mcp", AppId: new("app1"), DurationMs: 120,
			Objects: []*syncrunModel.Object{
				{
					Namespace: "prod", KubeKind: "Secret", KubeName: "kusec-app1-db", Op: syncrunModel.OpUnchanged,
					ContentHash: secretContentHash,
				},
				{
					Namespace: "prod", KubeKind: "ConfigMap", KubeName: "kusec-app1-guess", Op: syncrunModel.OpCreated,
					ContentHash: configMapContentHash, ChangedKeys: []string{"PIN"},
				},
			},
		}}},
		appListSvcMock{},
		sessionSvc,
	)
	auditUsecase := auditUsc.New(
		&auditListSvcMock{items: []*auditModel.Main{
			// item секрета: отпечатки значения — тот же value_hash, он остаётся
			{
				Id: 1, CreatedAt: startedAt, EntityType: auditModel.EntityItem, EntityId: "item1", AppId: new("app1"),
				Key: "DB_PASSWORD", Action: auditModel.ActionUpdate,
				Changes: []auditModel.Change{{
					Field:   "value",
					OldHash: itemOldValueHash, OldSize: new(int64(16)),
					NewHash: itemNewValueHash, NewSize: new(int64(32)),
				}},
			},
			// значение конфигмапа длиннее лимита: аудит хранит отпечаток и размер
			{
				Id: 2, CreatedAt: startedAt, EntityType: auditModel.EntityConfigItem, EntityId: "ci1", AppId: new("app1"),
				Key: "BIG_GUESS", Action: auditModel.ActionUpdate,
				Changes: []auditModel.Change{
					{Field: "description", Old: new("a"), New: new("b")},
					{
						Field:   "value",
						OldHash: configOldValueHash, OldSize: new(int64(5000)),
						NewHash: configNewValueHash, NewSize: new(int64(6000)),
						Truncated: true,
					},
				},
			},
		}},
		sessionSvc,
	)

	h := New(sessionSvc, apikeyUsecase, nil, nil, nil, nil, nil, nil, nil, auditUsecase, syncrunUsecase)

	httpSrv := httptest.NewServer(h.HTTPHandler())
	defer httpSrv.Close()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	defer clSession.Close()

	call := func(name string, args map[string]any) string {
		t.Helper()
		res, err := clSession.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
		require.NoError(t, err)
		raw, err := json.Marshal(res)
		require.NoError(t, err)
		require.False(t, res.IsError, string(raw))
		return string(raw)
	}

	forbidden := []string{secretContentHash, configMapContentHash, configOldValueHash, configNewValueHash}

	// sync_run_list: и один запуск с объектами, и список
	for name, args := range map[string]map[string]any{
		"запуск по id": {"id": "run1"},
		"список":       {},
	} {
		raw := call("sync_run_list", args)

		assert.Contains(t, raw, "run1", name)
		for _, hash := range forbidden {
			assert.NotContains(t, raw, hash, name)
		}
		assert.NotContains(t, raw, "content_hash", name)
	}

	// остальное об объектах запуска агенту по-прежнему видно
	raw := call("sync_run_list", map[string]any{"id": "run1"})
	for _, want := range []string{"kusec-app1-db", "kusec-app1-guess", `"ConfigMap"`, `"created"`, `"unchanged"`, `"PIN"`} {
		assert.Contains(t, raw, want)
	}

	// audit_list: отпечатков значения конфигмапа нет, размеры остались;
	// отпечатки значения item-а секрета на месте
	raw = call("audit_list", map[string]any{})
	for _, hash := range forbidden {
		assert.NotContains(t, raw, hash)
	}
	assert.Contains(t, raw, itemOldValueHash)
	assert.Contains(t, raw, itemNewValueHash)

	var auditOut struct {
		StructuredContent AuditListOut `json:"structuredContent"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &auditOut))
	require.Len(t, auditOut.StructuredContent.Results, 2)

	configChanges := auditOut.StructuredContent.Results[1].Changes
	require.Len(t, configChanges, 2)
	assert.Equal(t, AuditChangeOut{Field: "description", Old: new("a"), New: new("b")}, configChanges[0])
	assert.Equal(t, AuditChangeOut{
		Field: "value", OldSize: new(int64(5000)), NewSize: new(int64(6000)), Truncated: true,
	}, configChanges[1])

	itemChanges := auditOut.StructuredContent.Results[0].Changes
	require.Len(t, itemChanges, 1)
	assert.Equal(t, itemOldValueHash, itemChanges[0].OldHash)
	assert.Equal(t, itemNewValueHash, itemChanges[0].NewHash)
}

// TestToolSchemasHaveNoContentFingerprints — ни один инструмент не обещает
// агенту отпечаток содержимого объекта. Из полей с отпечатками разрешены
// только отпечатки значения item-а секрета (value_hash и его же пара в
// аудите): запись известных значений в секрет ограничена по частоте.
// Новое поле *_hash нужно сначала сверить с literal_limit.go.
func TestToolSchemasHaveNoContentFingerprints(t *testing.T) {
	t.Parallel()

	key, keyHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	sessionSvc := sessionService.New("test-secret")
	apikeyUsecase := apikeyUsc.New(
		&apikeySvcMock{byHash: map[string]*apikeyModel.Main{
			keyHash: {Id: "k1", UsrId: 10, Active: true, Scope: "mcp_only"},
		}},
		&usrSvcMock{usrs: map[int64]*usrModel.Main{10: {Id: 10, Active: true, IsAdmin: true}}},
		nil,
		txmStub{},
		auditRecStub{},
		nil,
	)
	h := New(sessionSvc, apikeyUsecase, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	httpSrv := httptest.NewServer(h.HTTPHandler())
	defer httpSrv.Close()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	defer clSession.Close()

	tools, err := clSession.ListTools(t.Context(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, tools.Tools)

	allowedHashFields := map[string]bool{"value_hash": true, "old_hash": true, "new_hash": true}

	seen := map[string]bool{}
	for _, tool := range tools.Tools {
		seen[tool.Name] = true

		raw, err := json.Marshal(tool.OutputSchema)
		require.NoError(t, err)

		var schema any
		require.NoError(t, json.Unmarshal(raw, &schema))

		for _, field := range schemaPropertyNames(schema) {
			lower := strings.ToLower(field)
			if !strings.Contains(lower, "hash") && !strings.Contains(lower, "fingerprint") && !strings.Contains(lower, "digest") {
				continue
			}
			assert.True(t, allowedHashFields[field], "инструмент %s отдаёт поле с отпечатком: %s", tool.Name, field)
		}

		// описание инструмента не обещает отпечаток содержимого объекта
		assert.NotContains(t, tool.Description, "content_hash", tool.Name)
		assert.NotContains(t, tool.Description, "отпечаток содержимого", tool.Name)
	}
	require.True(t, seen["sync_run_list"])
	require.True(t, seen["audit_list"])

	assert.NotContains(t, serverInstructions, "content_hash")
	assert.NotContains(t, serverInstructions, "отпечаток содержимого")
}

// schemaPropertyNames — имена всех свойств JSON-схемы, включая вложенные.
func schemaPropertyNames(schema any) []string {
	var names []string
	switch v := schema.(type) {
	case map[string]any:
		if props, ok := v["properties"].(map[string]any); ok {
			for name := range props {
				names = append(names, name)
			}
		}
		for _, child := range v {
			names = append(names, schemaPropertyNames(child)...)
		}
	case []any:
		for _, child := range v {
			names = append(names, schemaPropertyNames(child)...)
		}
	}
	return names
}
