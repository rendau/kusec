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
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	itemUsc "github.com/rendau/kusec/internal/usecase/item"
)

// ── Моки item usecase ───────────────────────────────────

type itemSvcMock struct {
	items map[string]*itemModel.Main
}

func (m *itemSvcMock) List(_ context.Context, _ *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
	result := make([]*itemModel.Main, 0, len(m.items))
	for _, item := range m.items {
		result = append(result, item)
	}
	return result, int64(len(result)), nil
}

func (m *itemSvcMock) Get(_ context.Context, id string, errNE bool) (*itemModel.Main, bool, error) {
	item, ok := m.items[id]
	if !ok && errNE {
		return nil, false, errs.ObjectNotFound
	}
	return item, ok, nil
}

func (m *itemSvcMock) Create(_ context.Context, _ *itemModel.Edit) (string, error) {
	return "", errs.NotImplemented
}

func (m *itemSvcMock) Update(_ context.Context, _ string, _ *itemModel.Edit) error { return nil }
func (m *itemSvcMock) Delete(_ context.Context, _ string) error                    { return nil }

type secretSvcMock struct {
	secrets map[string]*secretModel.Main
}

func (m *secretSvcMock) Get(_ context.Context, id string, errNE bool) (*secretModel.Main, bool, error) {
	sec, ok := m.secrets[id]
	if !ok && errNE {
		return nil, false, errs.ObjectNotFound
	}
	return sec, ok, nil
}

func (m *secretSvcMock) List(_ context.Context, _ *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
	result := make([]*secretModel.Main, 0, len(m.secrets))
	for _, sec := range m.secrets {
		result = append(result, sec)
	}
	return result, int64(len(result)), nil
}

// authTransport добавляет api-ключ в каждый HTTP-запрос MCP-клиента.
type authTransport struct {
	key string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.key)
	return http.DefaultTransport.RoundTrip(req)
}

// TestMaskingE2E — сквозная проверка маскирования: реальный streamable HTTP
// endpoint + MCP-клиент; значение секрета не должно появляться в сыром ответе
// инструментов чтения.
func TestMaskingE2E(t *testing.T) {
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

	h := New(sessionSvc, apikeyUsecase, nil, nil, itemUsecase, nil, nil, nil)

	httpSrv := httptest.NewServer(h.HTTPHandler())
	defer httpSrv.Close()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "0.0.1"}, nil)
	clSession, err := client.Connect(t.Context(), &mcpsdk.StreamableClientTransport{
		Endpoint:   httpSrv.URL,
		HTTPClient: &http.Client{Transport: &authTransport{key: key}},
	}, nil)
	require.NoError(t, err)
	defer clSession.Close()

	expectedMask := maskValue(secretValue)

	for name, args := range map[string]map[string]any{
		"get_item":  {"id": "item1"},
		"list_item": {"secret_id": "sec1"},
	} {
		res, err := clSession.CallTool(t.Context(), &mcpsdk.CallToolParams{Name: name, Arguments: args})
		require.NoError(t, err, name)
		require.False(t, res.IsError, name)

		// проверяем весь сырой ответ инструмента, а не отдельные поля
		raw, err := json.Marshal(res)
		require.NoError(t, err, name)

		assert.NotContains(t, string(raw), secretValue, "%s: значение секрета утекло в ответ", name)
		assert.Contains(t, string(raw), expectedMask.Sha256, "%s: нет усечённого sha256", name)
		assert.Contains(t, string(raw), `"value_chars"`, name)
	}
}
