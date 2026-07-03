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

	apikeyModel "github.com/mechta-market/kusec/internal/domain/apikey/model"
	apikeyService "github.com/mechta-market/kusec/internal/domain/apikey/service"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	sessionService "github.com/mechta-market/kusec/internal/domain/session/service"
	usrModel "github.com/mechta-market/kusec/internal/domain/usr/model"
	"github.com/mechta-market/kusec/internal/errs"
	kubeSvc "github.com/mechta-market/kusec/internal/service/kube"
	apikeyUsc "github.com/mechta-market/kusec/internal/usecase/apikey"
	itemUsc "github.com/mechta-market/kusec/internal/usecase/item"
	kubeUsc "github.com/mechta-market/kusec/internal/usecase/kube"
)

// ── Мок kube-сервиса ────────────────────────────────────

type kubeSvcMock struct {
	gotAppIds     []string
	secretsResult *kubeSvc.SyncResult
	configMaps    *kubeSvc.SyncResult
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

func (m *kubeSvcMock) ListClusterSecrets(_ context.Context, _ string) ([]*kubeSvc.ClusterSecret, bool, error) {
	return nil, false, nil
}

func (m *kubeSvcMock) ImportSecret(_ context.Context, _ string, _ kubeSvc.ImportRef, _ string) (*kubeSvc.ImportResult, error) {
	return nil, errs.NotImplemented
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
			keyHash: {Id: "k1", UsrId: 10, Active: true, McpOnly: true},
		}},
		&usrSvcMock{usrs: map[int64]*usrModel.Main{
			10: {Id: 10, Active: true, AppIds: []string{"app1"}},
		}},
		nil,
	)
	itemUsecase := itemUsc.New(
		&itemSvcMock{items: map[string]*itemModel.Main{
			"item1": {Id: "item1", SecretId: "sec1", Active: true, Key: "DB_PASSWORD", Value: secretValue},
		}},
		&secretSvcMock{secrets: map[string]*secretModel.Main{
			"sec1": {Id: "sec1", AppId: "app1", Active: true, SlugName: "db"},
		}},
		sessionSvc,
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

	h := New(sessionSvc, apikeyUsecase, nil, nil, itemUsecase, nil, nil, kubeUsecase)

	httpSrv := httptest.NewServer(h.HTTPHandler())
	defer httpSrv.Close()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	defer clSession.Close()

	// без текущего app и без all_apps — ошибка с подсказкой про use_app
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
