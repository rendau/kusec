package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyModel "github.com/mechta-market/kusec/internal/domain/apikey/model"
	apikeyService "github.com/mechta-market/kusec/internal/domain/apikey/service"
	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	sessionService "github.com/mechta-market/kusec/internal/domain/session/service"
	usrModel "github.com/mechta-market/kusec/internal/domain/usr/model"
	"github.com/mechta-market/kusec/internal/errs"
	apikeyUsc "github.com/mechta-market/kusec/internal/usecase/apikey"
)

// ── Моки apikey usecase ─────────────────────────────────

type apikeySvcMock struct {
	byHash map[string]*apikeyModel.Main
}

func (m *apikeySvcMock) List(_ context.Context, _ *apikeyModel.ListReq) ([]*apikeyModel.Main, int64, error) {
	return nil, 0, nil
}

func (m *apikeySvcMock) Get(_ context.Context, _ string, _ bool) (*apikeyModel.Main, bool, error) {
	return nil, false, nil
}

func (m *apikeySvcMock) GetByKeyHash(_ context.Context, keyHash string) (*apikeyModel.Main, bool, error) {
	item, ok := m.byHash[keyHash]
	return item, ok, nil
}

func (m *apikeySvcMock) Create(_ context.Context, _ *apikeyModel.Edit) (string, error) {
	return "", errs.NotImplemented
}

func (m *apikeySvcMock) Update(_ context.Context, _ string, _ *apikeyModel.Edit) error { return nil }
func (m *apikeySvcMock) TouchLastUsed(_ context.Context, _ string) error               { return nil }
func (m *apikeySvcMock) Delete(_ context.Context, _ string) error                      { return nil }

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

// ── Тесты ───────────────────────────────────────────────

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

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

	h := New(sessionSvc, apikeyUsecase, nil, nil, nil, nil, nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true

		session, _ := r.Context().Value(ctxKeySession).(*sessionModel.Session)
		require.NotNil(t, session)
		assert.Equal(t, int64(10), session.Id)
		assert.Equal(t, []string{"app1"}, session.AppIds)

		gotHash, _ := r.Context().Value(ctxKeyHash).(string)
		assert.Equal(t, keyHash, gotHash)
	})

	mw := h.authMiddleware(next)

	// без ключа — 401
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, nextCalled)

	// с неизвестным ключом — 401
	badKey, _, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+badKey)
	rec = httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, nextCalled)

	// с валидным ключом (mcp_only) — пропускает и кладёт сессию в контекст
	req = httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	rec = httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
	assert.True(t, nextCalled)
}

func TestBearerToken(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abc", bearerToken("Bearer abc"))
	assert.Equal(t, "abc", bearerToken("bearer abc"))
	assert.Equal(t, "abc", bearerToken("abc"))
	assert.Equal(t, "", bearerToken(""))
	assert.Equal(t, "", bearerToken("Basic abc"))
}
