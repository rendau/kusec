package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

func testJwt(t *testing.T, exp time.Time) string {
	t.Helper()

	payload, err := json.Marshal(map[string]any{"exp": exp.Unix()})
	require.NoError(t, err)

	return "h." + base64.RawURLEncoding.EncodeToString(payload) + ".s"
}

func TestParseError(t *testing.T) {
	t.Parallel()

	apiErr := parseError(400, []byte(`{"code":"object_not_found","message":"нет такого"}`))
	assert.Equal(t, "object_not_found", apiErr.Code)
	assert.Equal(t, "нет такого", apiErr.Message)

	// неразобранное тело не попадает в ошибку
	apiErr = parseError(404, []byte(`service path not found, secret leak`))
	assert.Equal(t, "http_404", apiErr.Code)
	assert.NotContains(t, apiErr.Error(), "secret leak")
}

func TestClient_LoginAndAuthRetry(t *testing.T) {
	t.Parallel()

	logins := 0
	tokens := []string{}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /usr/login", func(w http.ResponseWriter, r *http.Request) {
		logins++
		jwt := testJwt(t, time.Now().Add(time.Hour))
		tokens = append(tokens, jwt)
		_ = json.NewEncoder(w).Encode(model.LoginRep{Jwt: jwt, RefreshToken: "rt-" + jwt})
	})

	appCalls := 0
	mux.HandleFunc("GET /app/id1", func(w http.ResponseWriter, r *http.Request) {
		appCalls++
		// первый запрос отвергаем как неавторизованный — клиент должен
		// переавторизоваться и повторить
		if appCalls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(model.ErrorRep{Code: "not_authorized"})
			return
		}
		require.Equal(t, "Bearer "+tokens[len(tokens)-1], r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(model.App{ID: "id1", SlugName: "demo"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "", "usr", "pass", "", false)

	app, err := c.AppGet(context.Background(), "id1")
	require.NoError(t, err)
	assert.Equal(t, "demo", app.SlugName)
	assert.Equal(t, 2, logins)
	assert.Equal(t, 2, appCalls)
}

func TestClient_RefreshTokenFlow(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /usr/token/refresh", func(w http.ResponseWriter, r *http.Request) {
		req := model.RefreshTokenReq{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "refresh-1", req.RefreshToken)

		_ = json.NewEncoder(w).Encode(model.LoginRep{
			Jwt:          testJwt(t, time.Now().Add(time.Hour)),
			RefreshToken: "refresh-2",
		})
	})
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		require.Equal(t, "0", r.URL.Query().Get("list_params.page"))
		require.Equal(t, "100", r.URL.Query().Get("list_params.page_size"))

		_ = json.NewEncoder(w).Encode(model.AppListRep{
			PaginationInfo: model.PaginationInfo{TotalCount: 1},
			Results:        []model.App{{ID: "id1"}},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "", "", "", "refresh-1", false)

	rep, err := c.AppList(context.Background(), model.AppListReq{})
	require.NoError(t, err)
	require.Len(t, rep.Results, 1)
	assert.Equal(t, int64(1), int64(rep.PaginationInfo.TotalCount))

	// refresh-токен ротировался
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.Equal(t, "refresh-2", c.refreshToken)
}

func TestClient_ApiKeyAuth(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /usr/login", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("с API-ключом логин вызываться не должен")
	})
	mux.HandleFunc("GET /app/id1", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer ksk_test-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(model.App{ID: "id1"})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "ksk_test-key", "", "", "", false)

	app, err := c.AppGet(context.Background(), "id1")
	require.NoError(t, err)
	assert.Equal(t, "id1", app.ID)
}

func TestInt64Str(t *testing.T) {
	t.Parallel()

	obj := struct {
		A model.Int64Str `json:"a"`
		B model.Int64Str `json:"b"`
	}{}

	// protojson отдаёт int64 строкой, но поддерживаем и число
	require.NoError(t, json.Unmarshal([]byte(`{"a":"42","b":7}`), &obj))
	assert.Equal(t, int64(42), int64(obj.A))
	assert.Equal(t, int64(7), int64(obj.B))
}
