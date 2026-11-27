package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	oauthUsc "github.com/rendau/kusec/internal/usecase/oauth"
)

const testFailDuration = 50 * time.Millisecond

func TestAuthGuard_SlowDown(t *testing.T) {
	t.Parallel()

	g := &authGuard{minFailDuration: 80 * time.Millisecond}

	started := time.Now()
	g.slowDown(context.Background(), started)
	assert.GreaterOrEqual(t, time.Since(started), 80*time.Millisecond)

	// проверка уже шла дольше — без лишнего ожидания
	before := time.Now()
	g.slowDown(context.Background(), time.Now().Add(-time.Second))
	assert.Less(t, time.Since(before), 20*time.Millisecond)

	// отменённый запрос не ждёт
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before = time.Now()
	g.slowDown(ctx, time.Now())
	assert.Less(t, time.Since(before), 20*time.Millisecond)
}

func TestAuthMiddleware_FixedFailTime(t *testing.T) {
	t.Parallel()

	key, keyHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	disabledKey, disabledHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	unknownKey, _, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	apikeyUsecase := apikeyUsc.New(
		&apikeySvcMock{byHash: map[string]*apikeyModel.Main{
			keyHash:      {Id: "k1", UsrId: 10, Active: true, Scope: "mcp_only"},
			disabledHash: {Id: "k2", UsrId: 10, Active: false, Scope: "mcp_only"},
		}},
		&usrSvcMock{usrs: map[int64]*usrModel.Main{10: {Id: 10, Active: true}}},
		nil,
		txmStub{},
		auditRecStub{},
		nil,
	)

	h := New(nil, apikeyUsecase, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h.guard = &authGuard{minFailDuration: testFailDuration}

	mw := h.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), nil)

	call := func(credential string) (int, time.Duration) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		rec := httptest.NewRecorder()
		started := time.Now()
		mw.ServeHTTP(rec, req)
		return rec.Code, time.Since(started)
	}

	// отказ по любой причине — не раньше minFailDuration: строка не нашего
	// формата (до базы), ключа нет, ключ отключён
	for name, credential := range map[string]string{
		"garbage":  "garbage",
		"unknown":  unknownKey,
		"disabled": disabledKey,
	} {
		code, took := call(credential)
		assert.Equal(t, http.StatusUnauthorized, code, name)
		assert.GreaterOrEqual(t, took, testFailDuration, name)
	}

	// без ключа — шаг OAuth-входа, проверять нечего: сразу
	code, took := call("")
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Less(t, took, testFailDuration)

	// верный ключ — без задержки, сколько бы отказов ни было до него
	for range 30 {
		code, _ = call(unknownKey + "x")
		assert.Equal(t, http.StatusUnauthorized, code)
	}
	code, took = call(key)
	assert.Equal(t, http.StatusOK, code)
	assert.Less(t, took, testFailDuration)
}

func TestOAuthGuardedError(t *testing.T) {
	t.Parallel()

	e := &oauthEndpoints{guard: &authGuard{minFailDuration: testFailDuration}}

	for _, err := range []error{
		&oauthUsc.Error{Code: oauthUsc.ErrInvalidGrant},
		&oauthUsc.Error{Code: oauthUsc.ErrInvalidClient},
		errors.New("db down"),
	} {
		req := httptest.NewRequest(http.MethodPost, "/token", nil)
		rec := httptest.NewRecorder()
		started := time.Now()
		e.writeGuardedError(rec, req, started, err)
		assert.GreaterOrEqual(t, time.Since(started), testFailDuration, err.Error())
		assert.NotEqual(t, http.StatusOK, rec.Code)
	}
}
