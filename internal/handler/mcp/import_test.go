package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
	kubeSvc "github.com/rendau/kusec/internal/service/kube"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	appUsc "github.com/rendau/kusec/internal/usecase/app"
	itemUsc "github.com/rendau/kusec/internal/usecase/item"
	kubeUsc "github.com/rendau/kusec/internal/usecase/kube"
	secretUsc "github.com/rendau/kusec/internal/usecase/secret"
)

// ── Моки app usecase ────────────────────────────────────

type appSvcMock struct {
	apps map[string]*appModel.Main
}

func (m *appSvcMock) List(_ context.Context, _ *appModel.ListReq) ([]*appModel.Main, int64, error) {
	result := lo.Values(m.apps)
	return result, int64(len(result)), nil
}

func (m *appSvcMock) Get(_ context.Context, id string, errNE bool) (*appModel.Main, bool, error) {
	app, ok := m.apps[id]
	if !ok && errNE {
		return nil, false, errs.ObjectNotFound
	}
	return app, ok, nil
}

func (m *appSvcMock) Create(_ context.Context, _ *appModel.Edit) (string, error) {
	return "", errs.NotImplemented
}

func (m *appSvcMock) Update(_ context.Context, _ string, _ *appModel.Edit) error { return nil }
func (m *appSvcMock) Delete(_ context.Context, _ string) error                   { return nil }

// ── Дополнение моков secret/audit до интерфейсов secret usecase ──

// created — секреты, созданные через Create (в порядке вызовов).
func (m *secretSvcMock) Create(_ context.Context, obj *secretModel.Edit) (string, error) {
	id := "sec-new-" + *obj.SlugName
	m.secrets[id] = &secretModel.Main{
		Id:        id,
		AppId:     *obj.AppId,
		Active:    true,
		SlugName:  *obj.SlugName,
		ExactSlug: obj.ExactSlug != nil && *obj.ExactSlug,
	}
	return id, nil
}

func (m *secretSvcMock) Update(_ context.Context, id string, obj *secretModel.Edit) error {
	if obj.ExactSlug != nil {
		m.secrets[id].ExactSlug = *obj.ExactSlug
	}
	return nil
}

func (m *secretSvcMock) Delete(_ context.Context, _ string) error { return nil }

func (auditRecStub) NewBatchId() string { return "batch-test" }

func (auditRecStub) RecordApp(context.Context, *appModel.Main, *appModel.Main, string, *string) error {
	return nil
}

func (auditRecStub) RecordSecret(context.Context, *secretModel.Main, *secretModel.Main, string, *string) error {
	return nil
}

func (auditRecStub) RecordConfigMap(context.Context, *configmapModel.Main, *configmapModel.Main, string, *string) error {
	return nil
}

func (auditRecStub) RecordConfigItem(context.Context, *configitemModel.Main, *configitemModel.Main, string, *string) error {
	return nil
}

// ── Стенд: админский и обычный ключи ────────────────────

const (
	adminUsrId  int64 = 10
	memberUsrId int64 = 20
)

type importEnv struct {
	h         *Handler
	adminKey  string
	memberKey string
	kubeMock  *kubeSvcMock
	secretSvc *secretSvcMock
	itemSvc   *itemSvcMock
}

// newImportEnv собирает MCP-хендлер с реальными usecase-ами поверх моков:
// app "app1" (slug billing), секрет sec1 с одним заполненным item-ом.
// adminKey принадлежит админу, memberKey — пользователю с доступом к app1.
func newImportEnv(t *testing.T) *importEnv {
	t.Helper()

	adminKey, adminHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	memberKey, memberHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	sessionSvc := sessionService.New("test-secret")
	apikeyUsecase := apikeyUsc.New(
		&apikeySvcMock{byHash: map[string]*apikeyModel.Main{
			adminHash:  {Id: "k-admin", UsrId: adminUsrId, Active: true, Scope: "mcp_only"},
			memberHash: {Id: "k-member", UsrId: memberUsrId, Active: true, Scope: "mcp_only"},
		}},
		&usrSvcMock{usrs: map[int64]*usrModel.Main{
			adminUsrId:  {Id: adminUsrId, Active: true, IsAdmin: true},
			memberUsrId: {Id: memberUsrId, Active: true, AppIds: []string{"app1"}},
		}},
		nil,
		txmStub{},
		auditRecStub{},
	)

	appSvc := &appSvcMock{apps: map[string]*appModel.Main{
		"app1": {Id: "app1", Active: true, Namespace: "billing", Name: "Billing", SlugName: "billing"},
	}}
	secretSvc := &secretSvcMock{secrets: map[string]*secretModel.Main{
		"sec1": {Id: "sec1", AppId: "app1", Active: true, SlugName: "db"},
	}}
	itemSvc := &itemSvcMock{items: map[string]*itemModel.Main{
		"item1": {Id: "item1", SecretId: "sec1", Active: true, Key: "PG_USER", Value: "app"},
	}}
	kubeMock := &kubeSvcMock{}

	appUsecase := appUsc.New(appSvc, nil, nil, nil, nil, sessionSvc, txmStub{}, auditRecStub{})
	secretUsecase := secretUsc.New(secretSvc, appSvc, itemSvc, sessionSvc, txmStub{}, auditRecStub{})
	itemUsecase := itemUsc.New(itemSvc, secretSvc, sessionSvc, txmStub{}, auditRecStub{})
	kubeUsecase := kubeUsc.New(kubeMock, nil, nil, nil, sessionSvc)

	return &importEnv{
		h:         New(sessionSvc, apikeyUsecase, appUsecase, secretUsecase, itemUsecase, nil, nil, kubeUsecase, nil, nil),
		adminKey:  adminKey,
		memberKey: memberKey,
		kubeMock:  kubeMock,
		secretSvc: secretSvc,
		itemSvc:   itemSvc,
	}
}

// connect поднимает streamable HTTP endpoint и открывает MCP-сессию по ключу.
func (e *importEnv) connect(t *testing.T, key string) *mcpsdk.ClientSession {
	t.Helper()

	httpSrv := httptest.NewServer(e.h.HTTPHandler())
	t.Cleanup(httpSrv.Close)

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clSession.Close() })

	return clSession
}

// call вызывает инструмент и возвращает результат вместе с его сырым JSON.
func call(t *testing.T, s *mcpsdk.ClientSession, name string, args map[string]any) (*mcpsdk.CallToolResult, string) {
	t.Helper()

	res, err := s.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err, name)

	raw, err := json.Marshal(res)
	require.NoError(t, err)

	return res, string(raw)
}

// structuredKeys — имена полей верхнего уровня structuredContent ответа.
func structuredKeys(t *testing.T, res *mcpsdk.CallToolResult) []string {
	t.Helper()

	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))

	keys := lo.Keys(m)
	sort.Strings(keys)
	return keys
}

// ── import_secret ───────────────────────────────────────

func TestImportSecretE2E(t *testing.T) {
	t.Parallel()

	// значения «из кластера»: в MCP-путь они не попадают вовсе, но пусть
	// сентинели будут — проверяем ответ и ошибки на их отсутствие
	const clusterValue = "Cluster-Secret-Value-77"

	env := newImportEnv(t)

	var gotAppId, gotSlug string
	var gotRef kubeSvc.ImportRef
	var gotOpts kubeSvc.ImportOptions
	env.kubeMock.importFn = func(appId string, ref kubeSvc.ImportRef, secretSlug string, opts kubeSvc.ImportOptions) (*kubeSvc.ImportResult, error) {
		gotAppId, gotRef, gotSlug, gotOpts = appId, ref, secretSlug, opts
		if _, missing := opts.Keys["NOPE"]; missing {
			return nil, errs.ErrFull{Err: errs.InvalidRequest, Desc: "keys not found in cluster secret: NOPE"}
		}
		return &kubeSvc.ImportResult{
			ObjectId:      "sec1",
			Slug:          secretSlug,
			ObjectCreated: false,
			KubeType:      "",
			KubeName:      "kusec-billing-db",
			CreatedKeys:   []string{"DB_USER"},
			FilledKeys:    []string{"POSTGRES_PASSWORD"},
			UpdatedKeys:   []string{},
			SkippedKeys:   []string{"PG_DSN"},
			CreatedItems:  1,
			UpdatedItems:  1,
		}, nil
	}

	admin := env.connect(t, env.adminKey)

	// админ: app по slug, keys с переименованием, overwrite по умолчанию false
	res, raw := call(t, admin, "import_secret", map[string]any{
		"app":         "billing",
		"namespace":   "billing",
		"name":        "billing-db",
		"secret_slug": "db",
		"keys":        map[string]any{"POSTGRES_PASSWORD": "", "POSTGRES_USER": "DB_USER"},
	})
	require.False(t, res.IsError, raw)

	assert.Equal(t, "app1", gotAppId)
	assert.Equal(t, kubeSvc.ImportRef{Namespace: "billing", Name: "billing-db"}, gotRef)
	assert.Equal(t, "db", gotSlug)
	assert.Equal(t, map[string]string{"POSTGRES_PASSWORD": "", "POSTGRES_USER": "DB_USER"}, gotOpts.Keys)
	assert.False(t, gotOpts.Overwrite)

	// контракт ответа: ровно эти поля, списки имён по категориям, без значений
	assert.Equal(t, []string{
		"created", "filled", "kube_secret_name", "kube_type",
		"secret_created", "secret_id", "secret_slug", "skipped", "updated",
	}, structuredKeys(t, res))
	assert.Contains(t, raw, `"created":["DB_USER"]`)
	assert.Contains(t, raw, `"filled":["POSTGRES_PASSWORD"]`)
	assert.Contains(t, raw, `"updated":[]`)
	assert.Contains(t, raw, `"skipped":["PG_DSN"]`)
	assert.Contains(t, raw, `"kube_secret_name":"kusec-billing-db"`)
	assert.NotContains(t, raw, clusterValue)
	assert.NotContains(t, raw, `"value"`)

	// overwrite=true доходит до сервиса
	res, raw = call(t, admin, "import_secret", map[string]any{
		"app": "app1", "namespace": "billing", "name": "billing-db", "secret_slug": "db", "overwrite": true,
	})
	require.False(t, res.IsError, raw)
	assert.True(t, gotOpts.Overwrite)
	assert.Nil(t, gotOpts.Keys)

	// ошибка сервиса про недостающий ключ доходит до агента с именем ключа
	res, raw = call(t, admin, "import_secret", map[string]any{
		"app": "billing", "namespace": "billing", "name": "billing-db", "secret_slug": "db",
		"keys": map[string]any{"NOPE": ""},
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "NOPE")
	assert.NotContains(t, raw, clusterValue)

	// без обязательных полей — ошибка до похода в usecase
	for name, args := range map[string]map[string]any{
		"no app":       {"namespace": "billing", "name": "billing-db", "secret_slug": "db"},
		"no namespace": {"app": "billing", "name": "billing-db", "secret_slug": "db"},
		"no slug":      {"app": "billing", "namespace": "billing", "name": "billing-db"},
	} {
		res, _ = call(t, admin, "import_secret", args)
		assert.True(t, res.IsError, name)
	}

	// не-админ: usecase отвечает no_permission, сервис не вызывается
	gotAppId = ""
	member := env.connect(t, env.memberKey)
	res, raw = call(t, member, "import_secret", map[string]any{
		"app": "billing", "namespace": "billing", "name": "billing-db", "secret_slug": "db",
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
	assert.Empty(t, gotAppId)
}

// ── list_cluster_secret ─────────────────────────────────

func TestListClusterSecretE2E(t *testing.T) {
	t.Parallel()

	env := newImportEnv(t)
	env.kubeMock.clusterSecrets = []*kubeSvc.ClusterSecret{
		{Namespace: "billing", Name: "billing-db", Type: "", Keys: []string{"POSTGRES_PASSWORD", "POSTGRES_USER"}, Managed: false},
		{Namespace: "billing", Name: "kusec-billing-api", Type: "kubernetes.io/basic-auth", Keys: []string{"password", "username"}, Managed: true},
	}

	admin := env.connect(t, env.adminKey)

	res, raw := call(t, admin, "list_cluster_secret", map[string]any{"namespace": "billing"})
	require.False(t, res.IsError, raw)
	assert.Equal(t, "billing", env.kubeMock.gotNamespace)

	assert.Equal(t, []string{"in_cluster", "secrets"}, structuredKeys(t, res))
	assert.Contains(t, raw, `"in_cluster":true`)
	assert.Contains(t, raw, `"keys":["POSTGRES_PASSWORD","POSTGRES_USER"]`)
	assert.Contains(t, raw, `"managed":true`)
	assert.Contains(t, raw, `"type":"kubernetes.io/basic-auth"`)
	assert.NotContains(t, raw, `"value`, "в списке секретов кластера нет ни значений, ни их масок")

	// не-админ — no_permission
	member := env.connect(t, env.memberKey)
	res, raw = call(t, member, "list_cluster_secret", map[string]any{})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
}

// ── exact_slug через MCP ────────────────────────────────

func TestExactSlugE2E(t *testing.T) {
	t.Parallel()

	env := newImportEnv(t)
	admin := env.connect(t, env.adminKey)
	member := env.connect(t, env.memberKey)

	// админ создаёт секрет с exact_slug=true
	res, raw := call(t, admin, "create_secret", map[string]any{
		"app": "billing", "slug_name": "legacy-db", "exact_slug": true,
	})
	require.False(t, res.IsError, raw)
	require.Contains(t, env.secretSvc.secrets, "sec-new-legacy-db")
	assert.True(t, env.secretSvc.secrets["sec-new-legacy-db"].ExactSlug)

	// не-админ с exact_slug=true — no_permission, секрет не создан
	res, raw = call(t, member, "create_secret", map[string]any{
		"app": "billing", "slug_name": "legacy-cache", "exact_slug": true,
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
	assert.NotContains(t, env.secretSvc.secrets, "sec-new-legacy-cache")

	// не-админ без exact_slug — как раньше, можно
	res, raw = call(t, member, "create_secret", map[string]any{"app": "billing", "slug_name": "cache"})
	require.False(t, res.IsError, raw)
	assert.False(t, env.secretSvc.secrets["sec-new-cache"].ExactSlug)

	// update_secret exact_slug: не-админ — no_permission
	res, raw = call(t, member, "update_secret", map[string]any{"id": "sec1", "exact_slug": true})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")

	// админ, но у секрета есть активный item без значения — guard usecase
	env.itemSvc.items["item-empty"] = &itemModel.Main{Id: "item-empty", SecretId: "sec1", Active: true, Key: "PG_PASSWORD", Value: ""}
	res, raw = call(t, admin, "update_secret", map[string]any{"id": "sec1", "exact_slug": true})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "invalid_request")
	assert.Contains(t, raw, "PG_PASSWORD")
	assert.False(t, env.secretSvc.secrets["sec1"].ExactSlug)

	// значение заполнено (как после import_secret) — флаг включается
	env.itemSvc.items["item-empty"].Value = "filled-after-import"
	res, raw = call(t, admin, "update_secret", map[string]any{"id": "sec1", "exact_slug": true})
	require.False(t, res.IsError, raw)
	assert.True(t, env.secretSvc.secrets["sec1"].ExactSlug)
}

// ── import_secret_batch ─────────────────────────────────

// structuredResults — элементы results из structuredContent ответа как карты.
func structuredResults(t *testing.T, res *mcpsdk.CallToolResult) []map[string]any {
	t.Helper()

	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)

	var out struct {
		Results []map[string]any `json:"results"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out.Results
}

func TestImportSecretBatchE2E(t *testing.T) {
	t.Parallel()

	const clusterValue = "Cluster-Secret-Value-88"

	env := newImportEnv(t)

	var gotAppId string
	var gotSpecs []kubeSvc.ImportSpec
	env.kubeMock.importBatchFn = func(appId string, specs []kubeSvc.ImportSpec) ([]kubeSvc.ImportBatchItem, error) {
		gotAppId, gotSpecs = appId, specs
		return lo.Map(specs, func(spec kubeSvc.ImportSpec, _ int) kubeSvc.ImportBatchItem {
			item := kubeSvc.ImportBatchItem{Ref: spec.Ref, Slug: spec.Slug}
			if spec.Ref.Name == "missing" {
				item.Err = errs.ErrFull{Err: errs.ObjectNotFound, Desc: "get cluster secret " + spec.Ref.Namespace + "/missing: not found"}
				return item
			}
			item.Result = &kubeSvc.ImportResult{
				ObjectId:      "sec-" + spec.Slug,
				Slug:          spec.Slug,
				ObjectCreated: true,
				KubeName:      "kusec-billing-" + spec.Slug,
				CreatedKeys:   lo.Keys(spec.Options.Keys),
			}
			return item
		}), nil
	}

	admin := env.connect(t, env.adminKey)

	res, raw := call(t, admin, "import_secret_batch", map[string]any{
		"app":       "billing",
		"namespace": "billing",
		"overwrite": true,
		"secrets": []map[string]any{
			{"name": "billing-db", "secret_slug": "db", "keys": map[string]any{"POSTGRES_PASSWORD": ""}, "overwrite": false},
			{"name": "missing"},
			{"namespace": "other", "name": "cache"},
		},
	})
	require.False(t, res.IsError, raw)

	// дефолты пакета и переопределения по секретам
	assert.Equal(t, "app1", gotAppId)
	require.Len(t, gotSpecs, 3)
	assert.Equal(t, kubeSvc.ImportSpec{
		Ref:     kubeSvc.ImportRef{Namespace: "billing", Name: "billing-db"},
		Slug:    "db",
		Options: kubeSvc.ImportOptions{Keys: map[string]string{"POSTGRES_PASSWORD": ""}, Overwrite: false},
	}, gotSpecs[0])
	assert.Equal(t, kubeSvc.ImportSpec{
		Ref:     kubeSvc.ImportRef{Namespace: "billing", Name: "missing"},
		Slug:    "missing",
		Options: kubeSvc.ImportOptions{Overwrite: true},
	}, gotSpecs[1])
	assert.Equal(t, kubeSvc.ImportSpec{
		Ref:     kubeSvc.ImportRef{Namespace: "other", Name: "cache"},
		Slug:    "cache",
		Options: kubeSvc.ImportOptions{Overwrite: true},
	}, gotSpecs[2])

	// контракт ответа
	assert.Equal(t, []string{"failed", "ok", "results"}, structuredKeys(t, res))
	assert.Contains(t, raw, `"ok":2`)
	assert.Contains(t, raw, `"failed":1`)

	results := structuredResults(t, res)
	require.Len(t, results, 3)
	for _, r := range results {
		keys := lo.Keys(r)
		sort.Strings(keys)
		assert.Equal(t, []string{
			"created", "error", "filled", "kube_secret_name", "kube_type", "name", "namespace",
			"secret_created", "secret_id", "secret_slug", "skipped", "updated",
		}, keys)
	}

	assert.Equal(t, "", results[0]["error"])
	assert.Equal(t, "sec-db", results[0]["secret_id"])
	assert.Equal(t, []any{"POSTGRES_PASSWORD"}, results[0]["created"])

	// упавший секрет: источник, slug и причина; списки пустые, не null
	assert.Equal(t, "missing", results[1]["name"])
	assert.Equal(t, "missing", results[1]["secret_slug"])
	assert.Contains(t, results[1]["error"], "billing/missing")
	assert.Equal(t, "", results[1]["secret_id"])
	assert.Equal(t, []any{}, results[1]["created"])

	assert.Equal(t, "other", results[2]["namespace"])
	assert.Equal(t, "", results[2]["error"])

	assert.NotContains(t, raw, clusterValue)
	assert.NotContains(t, raw, `"value"`)

	// пустой пакет и секрет без name — invalid_request на весь вызов, сервис не вызывается
	gotSpecs = nil
	res, raw = call(t, admin, "import_secret_batch", map[string]any{"app": "billing", "secrets": []map[string]any{}})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "invalid_request")

	// пустой name доходит до usecase — ошибка указывает на элемент
	res, raw = call(t, admin, "import_secret_batch", map[string]any{
		"app": "billing", "namespace": "billing",
		"secrets": []map[string]any{{"name": "ok"}, {"name": "", "secret_slug": "no-name"}},
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "invalid_request")
	assert.Contains(t, raw, "secrets[1]")
	assert.Nil(t, gotSpecs)

	// элемент без свойства name отсекает JSON-схема инструмента ещё до хендлера
	res, raw = call(t, admin, "import_secret_batch", map[string]any{
		"app": "billing", "namespace": "billing",
		"secrets": []map[string]any{{"secret_slug": "no-name"}},
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "missing properties")
	assert.Contains(t, raw, "name")
	assert.Nil(t, gotSpecs)

	// не-админ — no_permission
	member := env.connect(t, env.memberKey)
	res, raw = call(t, member, "import_secret_batch", map[string]any{
		"app": "billing", "namespace": "billing", "secrets": []map[string]any{{"name": "billing-db"}},
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
	assert.Nil(t, gotSpecs)
}

// ── configmap-ы: list_cluster_configmap / import_configmap / batch ──

func TestListClusterConfigMapE2E(t *testing.T) {
	t.Parallel()

	env := newImportEnv(t)
	env.kubeMock.clusterConfigMaps = []*kubeSvc.ClusterConfigMap{
		{Namespace: "billing", Name: "billing-config", Keys: []string{"PG_HOST", "PG_PORT"}, Managed: false},
		{Namespace: "billing", Name: "kusec-billing-api", Keys: []string{"TZ"}, Managed: true},
	}

	admin := env.connect(t, env.adminKey)

	res, raw := call(t, admin, "list_cluster_configmap", map[string]any{"namespace": "billing"})
	require.False(t, res.IsError, raw)
	assert.Equal(t, "billing", env.kubeMock.gotNamespace)

	assert.Equal(t, []string{"configmaps", "in_cluster"}, structuredKeys(t, res))
	assert.Contains(t, raw, `"in_cluster":true`)
	assert.Contains(t, raw, `"keys":["PG_HOST","PG_PORT"]`)
	assert.Contains(t, raw, `"managed":true`)
	assert.NotContains(t, raw, `"value`)

	member := env.connect(t, env.memberKey)
	res, raw = call(t, member, "list_cluster_configmap", map[string]any{})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
}

func TestImportConfigMapE2E(t *testing.T) {
	t.Parallel()

	env := newImportEnv(t)

	var gotAppId, gotSlug string
	var gotRef kubeSvc.ImportRef
	var gotOpts kubeSvc.ImportOptions
	env.kubeMock.importFn = func(appId string, ref kubeSvc.ImportRef, slug string, opts kubeSvc.ImportOptions) (*kubeSvc.ImportResult, error) {
		gotAppId, gotRef, gotSlug, gotOpts = appId, ref, slug, opts
		return &kubeSvc.ImportResult{
			ObjectId:      "cm1",
			Slug:          slug,
			ObjectCreated: true,
			KubeName:      "kusec-billing-config",
			CreatedKeys:   []string{"DB_HOST", "PG_PORT"},
			SkippedKeys:   []string{"TZ"},
		}, nil
	}

	admin := env.connect(t, env.adminKey)

	res, raw := call(t, admin, "import_configmap", map[string]any{
		"app":            "billing",
		"namespace":      "billing",
		"name":           "billing-config",
		"configmap_slug": "config",
		"keys":           map[string]any{"PG_HOST": "DB_HOST", "PG_PORT": ""},
		"overwrite":      true,
	})
	require.False(t, res.IsError, raw)

	assert.Equal(t, "app1", gotAppId)
	assert.Equal(t, kubeSvc.ImportRef{Namespace: "billing", Name: "billing-config"}, gotRef)
	assert.Equal(t, "config", gotSlug)
	assert.Equal(t, map[string]string{"PG_HOST": "DB_HOST", "PG_PORT": ""}, gotOpts.Keys)
	assert.True(t, gotOpts.Overwrite)

	// контракт ответа: без kube_type, имена полей — configmap_*
	assert.Equal(t, []string{
		"configmap_created", "configmap_id", "configmap_slug", "created", "filled",
		"kube_configmap_name", "skipped", "updated",
	}, structuredKeys(t, res))
	assert.Contains(t, raw, `"configmap_id":"cm1"`)
	assert.Contains(t, raw, `"created":["DB_HOST","PG_PORT"]`)
	assert.Contains(t, raw, `"filled":[]`)
	assert.Contains(t, raw, `"skipped":["TZ"]`)
	assert.Contains(t, raw, `"kube_configmap_name":"kusec-billing-config"`)

	// без обязательных полей — ошибка
	res, _ = call(t, admin, "import_configmap", map[string]any{"app": "billing", "namespace": "billing", "name": "billing-config"})
	assert.True(t, res.IsError)

	// не-админ — no_permission, сервис не вызывается
	gotAppId = ""
	member := env.connect(t, env.memberKey)
	res, raw = call(t, member, "import_configmap", map[string]any{
		"app": "billing", "namespace": "billing", "name": "billing-config", "configmap_slug": "config",
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
	assert.Empty(t, gotAppId)
}

func TestImportConfigMapBatchE2E(t *testing.T) {
	t.Parallel()

	env := newImportEnv(t)

	var gotSpecs []kubeSvc.ImportSpec
	env.kubeMock.importBatchFn = func(_ string, specs []kubeSvc.ImportSpec) ([]kubeSvc.ImportBatchItem, error) {
		gotSpecs = specs
		return lo.Map(specs, func(spec kubeSvc.ImportSpec, _ int) kubeSvc.ImportBatchItem {
			item := kubeSvc.ImportBatchItem{Ref: spec.Ref, Slug: spec.Slug}
			if spec.Ref.Name == "missing" {
				item.Err = errs.ErrFull{Err: errs.ObjectNotFound, Desc: "get cluster configmap " + spec.Ref.Namespace + "/missing: not found"}
				return item
			}
			item.Result = &kubeSvc.ImportResult{
				ObjectId:      "cm-" + spec.Slug,
				Slug:          spec.Slug,
				ObjectCreated: true,
				KubeName:      "kusec-billing-" + spec.Slug,
				CreatedKeys:   []string{"K"},
			}
			return item
		}), nil
	}

	admin := env.connect(t, env.adminKey)

	res, raw := call(t, admin, "import_configmap_batch", map[string]any{
		"app":       "billing",
		"namespace": "billing",
		"configmaps": []map[string]any{
			{"name": "billing-config", "configmap_slug": "config", "overwrite": true},
			{"name": "missing"},
		},
	})
	require.False(t, res.IsError, raw)

	require.Len(t, gotSpecs, 2)
	assert.Equal(t, kubeSvc.ImportSpec{
		Ref:     kubeSvc.ImportRef{Namespace: "billing", Name: "billing-config"},
		Slug:    "config",
		Options: kubeSvc.ImportOptions{Overwrite: true},
	}, gotSpecs[0])
	assert.Equal(t, kubeSvc.ImportSpec{
		Ref:  kubeSvc.ImportRef{Namespace: "billing", Name: "missing"},
		Slug: "missing",
	}, gotSpecs[1])

	assert.Equal(t, []string{"failed", "ok", "results"}, structuredKeys(t, res))
	assert.Contains(t, raw, `"ok":1`)
	assert.Contains(t, raw, `"failed":1`)

	results := structuredResults(t, res)
	require.Len(t, results, 2)
	for _, r := range results {
		keys := lo.Keys(r)
		sort.Strings(keys)
		assert.Equal(t, []string{
			"configmap_created", "configmap_id", "configmap_slug", "created", "error", "filled",
			"kube_configmap_name", "name", "namespace", "skipped", "updated",
		}, keys)
	}
	assert.Equal(t, "", results[0]["error"])
	assert.Equal(t, "cm-config", results[0]["configmap_id"])
	assert.Equal(t, "missing", results[1]["configmap_slug"])
	assert.Contains(t, results[1]["error"], "billing/missing")
	assert.Equal(t, []any{}, results[1]["created"])

	// пустой name — invalid_request с индексом элемента
	gotSpecs = nil
	res, raw = call(t, admin, "import_configmap_batch", map[string]any{
		"app": "billing", "namespace": "billing",
		"configmaps": []map[string]any{{"name": "ok"}, {"name": ""}},
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "configmaps[1]")
	assert.Nil(t, gotSpecs)

	// не-админ — no_permission
	member := env.connect(t, env.memberKey)
	res, raw = call(t, member, "import_configmap_batch", map[string]any{
		"app": "billing", "namespace": "billing", "configmaps": []map[string]any{{"name": "billing-config"}},
	})
	require.True(t, res.IsError)
	assert.Contains(t, raw, "no_permission")
	assert.Nil(t, gotSpecs)
}
