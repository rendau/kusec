package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
	kubeSvc "github.com/rendau/kusec/internal/service/kube"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	itemUsc "github.com/rendau/kusec/internal/usecase/item"
	kubeUsc "github.com/rendau/kusec/internal/usecase/kube"
)

// ── Мок kube-сервиса ────────────────────────────────────

type kubeSvcMock struct {
	gotAppIds     []string
	secretsResult *kubeSvc.SyncResult
	configMaps    *kubeSvc.SyncResult

	// импорт / списки объектов кластера (import_test.go); importFn и
	// importBatchFn обслуживают и секреты, и configmap-ы
	importFn          func(appId string, ref kubeSvc.ImportRef, slug string, opts kubeSvc.ImportOptions) (*kubeSvc.ImportResult, error)
	importBatchFn     func(appId string, specs []kubeSvc.ImportSpec) ([]kubeSvc.ImportBatchItem, error)
	clusterSecrets    []*kubeSvc.ClusterSecret
	clusterConfigMaps []*kubeSvc.ClusterConfigMap
	gotNamespace      string
}

func (m *kubeSvcMock) Sync(_ context.Context, appIds []string) (*kubeSvc.SyncResult, *kubeSvc.SyncResult, error) {
	m.gotAppIds = appIds
	return m.secretsResult, m.configMaps, nil
}

func (m *kubeSvcMock) SyncSecrets(_ context.Context, _ []string) (*kubeSvc.SyncResult, error) {
	return nil, errs.NotImplemented
}

func (m *kubeSvcMock) SyncConfigMaps(_ context.Context, _ []string) (*kubeSvc.SyncResult, error) {
	return nil, errs.NotImplemented
}

func (m *kubeSvcMock) ListNamespaces(_ context.Context) ([]string, bool, error) {
	return nil, false, nil
}

func (m *kubeSvcMock) ListClusterSecrets(_ context.Context, namespace string) ([]*kubeSvc.ClusterSecret, bool, error) {
	m.gotNamespace = namespace
	return m.clusterSecrets, len(m.clusterSecrets) > 0, nil
}

func (m *kubeSvcMock) ImportSecret(_ context.Context, appId string, ref kubeSvc.ImportRef, secretSlug string, opts kubeSvc.ImportOptions) (*kubeSvc.ImportResult, error) {
	if m.importFn == nil {
		return nil, errs.NotImplemented
	}
	return m.importFn(appId, ref, secretSlug, opts)
}

func (m *kubeSvcMock) ImportSecrets(_ context.Context, appId string, specs []kubeSvc.ImportSpec) ([]kubeSvc.ImportBatchItem, error) {
	if m.importBatchFn == nil {
		return nil, errs.NotImplemented
	}
	return m.importBatchFn(appId, specs)
}

func (m *kubeSvcMock) ListClusterConfigMaps(_ context.Context, namespace string) ([]*kubeSvc.ClusterConfigMap, bool, error) {
	m.gotNamespace = namespace
	return m.clusterConfigMaps, len(m.clusterConfigMaps) > 0, nil
}

func (m *kubeSvcMock) ImportConfigMap(_ context.Context, appId string, ref kubeSvc.ImportRef, configMapSlug string, opts kubeSvc.ImportOptions) (*kubeSvc.ImportResult, error) {
	if m.importFn == nil {
		return nil, errs.NotImplemented
	}
	return m.importFn(appId, ref, configMapSlug, opts)
}

func (m *kubeSvcMock) ImportConfigMaps(_ context.Context, appId string, specs []kubeSvc.ImportSpec) ([]kubeSvc.ImportBatchItem, error) {
	if m.importBatchFn == nil {
		return nil, errs.NotImplemented
	}
	return m.importBatchFn(appId, specs)
}

func (m *kubeSvcMock) GetClusterSecret(_ context.Context, _, _ string) (*kubeSvc.ClusterResource, bool, bool, error) {
	return nil, false, false, nil
}

func (m *kubeSvcMock) GetClusterConfigMap(_ context.Context, _, _ string) (*kubeSvc.ClusterResource, bool, bool, error) {
	return nil, false, false, nil
}

// TestSyncE2E — сквозная проверка инструмента sync: область синхронизации
// ограничивается правами ключа, значения секретов вычищаются из ошибок sync.
func TestSyncE2E(t *testing.T) {
	t.Parallel()

	const secretValue = "Sup3r-Secret-Value-42"

	key, keyHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

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
	)
	itemUsecase := itemUsc.New(
		&itemSvcMock{items: map[string]*itemModel.Main{
			"item1": {Id: "item1", SecretId: "sec1", Active: true, Key: "DB_PASSWORD", Value: secretValue},
		}},
		&secretSvcMock{secrets: map[string]*secretModel.Main{
			"sec1": {Id: "sec1", AppId: "app1", Active: true, SlugName: "db"},
		}},
		sessionSvc,
		txmStub{},
		auditRecStub{},
	)
	kubeMock := &kubeSvcMock{
		secretsResult: &kubeSvc.SyncResult{
			Created:   []string{"prod/kusec-app1-db"},
			Unchanged: 2,
			// значение секрета в тексте ошибки должно быть вычищено
			Errors: []string{"secret item DB_PASSWORD: value " + secretValue + " rejected"},
		},
		configMaps: &kubeSvc.SyncResult{},
	}
	kubeUsecase := kubeUsc.New(kubeMock, nil, nil, nil, sessionSvc)

	h := New(sessionSvc, apikeyUsecase, nil, nil, itemUsecase, nil, nil, kubeUsecase, nil, nil)

	httpSrv := httptest.NewServer(h.HTTPHandler())
	defer httpSrv.Close()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	defer clSession.Close()

	// без app и без all_apps — ошибка с подсказкой указать область
	res, err := clSession.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "sync", Arguments: map[string]any{}})
	require.NoError(t, err)
	require.True(t, res.IsError)

	// get_item — значение попадает в реестр сессии для скраба
	res, err = clSession.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "get_item", Arguments: map[string]any{"id": "item1"}})
	require.NoError(t, err)
	require.False(t, res.IsError)

	// sync по всем доступным app
	res, err = clSession.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: "sync", Arguments: map[string]any{"all_apps": true}})
	require.NoError(t, err)
	require.False(t, res.IsError)

	// область синхронизации — только app-ы, доступные владельцу ключа
	assert.Equal(t, []string{"app1"}, kubeMock.gotAppIds)

	raw, err := json.Marshal(res)
	require.NoError(t, err)

	assert.Contains(t, string(raw), "prod/kusec-app1-db")
	assert.NotContains(t, string(raw), secretValue, "значение секрета утекло в ответ sync")
	assert.Contains(t, string(raw), "[REDACTED]")
}
