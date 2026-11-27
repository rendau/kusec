package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/rendau/kusec/internal/constant"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
)

// authMock — и UsrAuthI, и ApiKeyAuthI: отдаёт заготовленный ответ и
// запоминает, каким токеном его вызвали.
type authMock struct {
	session *sessionModel.Session
	err     error

	gotTokens []string
}

func (m *authMock) SessionFromToken(_ context.Context, token string) (*sessionModel.Session, error) {
	m.gotTokens = append(m.gotTokens, token)
	return m.session, m.err
}

func (m *authMock) SessionFromKey(_ context.Context, key string) (*sessionModel.Session, error) {
	m.gotTokens = append(m.gotTokens, key)
	return m.session, m.err
}

// Сессию по access-токену строит usr usecase (по живой записи пользователя),
// по ключу — apikey usecase. Отказ (токен не принят) — запрос идёт как
// неавторизованный, решение принимает usecase.
func TestGrpcInterceptorSession(t *testing.T) {
	t.Parallel()

	sessionSvc := sessionService.New("test-secret")

	usrSession := &sessionModel.Session{Id: 7, AppIds: []string{"app1"}, Source: constant.SourceUi}
	keySession := &sessionModel.Session{Id: 9, Admin: true, Source: constant.SourceApi, ApiKeyId: "k1"}

	cases := []struct {
		name          string
		authorization string
		usrAuth       *authMock
		keyAuth       *authMock
		wantSession   *sessionModel.Session // nil — неавторизованный запрос
		wantUsrTokens []string
		wantKeyTokens []string
	}{
		{
			name:    "без токена",
			usrAuth: &authMock{session: usrSession},
			keyAuth: &authMock{session: keySession},
		},
		{
			name:          "access-токен — сессию даёт usr usecase",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{session: usrSession},
			keyAuth:       &authMock{session: keySession},
			wantSession:   usrSession,
			wantUsrTokens: []string{"jwt-token"},
		},
		{
			name:          "токен без слова Bearer",
			authorization: "jwt-token",
			usrAuth:       &authMock{session: usrSession},
			keyAuth:       &authMock{session: keySession},
			wantSession:   usrSession,
			wantUsrTokens: []string{"jwt-token"},
		},
		{
			name:          "отказ: выключенный пользователь или отозванный токен",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{err: errs.NotAuthorized},
			keyAuth:       &authMock{session: keySession},
			wantUsrTokens: []string{"jwt-token"},
		},
		{
			name:          "отказ, обёрнутый по дороге",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{err: fmt.Errorf("usc: %w", errs.NotAuthorized)},
			keyAuth:       &authMock{session: keySession},
			wantUsrTokens: []string{"jwt-token"},
		},
		{
			name:          "пустая сессия без ошибки — не авторизован",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{session: &sessionModel.Session{}},
			keyAuth:       &authMock{session: keySession},
			wantUsrTokens: []string{"jwt-token"},
		},
		{
			name:          "api-ключ — сессию даёт apikey usecase",
			authorization: "Bearer " + constant.ApiKeyPrefix + "abc",
			usrAuth:       &authMock{session: usrSession},
			keyAuth:       &authMock{session: keySession},
			wantSession:   keySession,
			wantKeyTokens: []string{constant.ApiKeyPrefix + "abc"},
		},
		{
			name:          "api-ключ отвергнут",
			authorization: "Bearer " + constant.ApiKeyPrefix + "abc",
			usrAuth:       &authMock{session: usrSession},
			keyAuth:       &authMock{err: errs.NotAuthorized},
			wantKeyTokens: []string{constant.ApiKeyPrefix + "abc"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			interceptor := GrpcInterceptorSession(sessionSvc, tc.keyAuth, tc.usrAuth)

			ctx := t.Context()
			if tc.authorization != "" {
				ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", tc.authorization))
			}

			var got *sessionModel.Session
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/kusec_v1.App/List"},
				func(ctx context.Context, _ any) (any, error) {
					got = sessionSvc.FromContext(ctx)
					return nil, nil
				},
			)
			require.NoError(t, err)
			require.NotNil(t, got, "handler должен быть вызван")

			if tc.wantSession == nil {
				assert.False(t, got.IsAuthorized())
				assert.False(t, got.IsAdmin())
				_, all := got.AccessibleAppIds()
				assert.False(t, all)
			} else {
				assert.Equal(t, tc.wantSession, got)
			}
			assert.Equal(t, tc.wantUsrTokens, tc.usrAuth.gotTokens)
			assert.Equal(t, tc.wantKeyTokens, tc.keyAuth.gotTokens)
		})
	}
}

// Сбой проверки токена или ключа (база недоступна) — не отказ: запрос
// обрывается ошибкой service_not_available и до handler-а не доходит.
// Раньше он шёл дальше как неавторизованный, клиент получал not_authorized и
// выбрасывал рабочий токен — короткая недоступность Postgres разлогинивала
// всех. Подробности сбоя наружу не уходят.
func TestGrpcInterceptorSession_CheckFailure(t *testing.T) {
	t.Parallel()

	sessionSvc := sessionService.New("test-secret")

	const pgText = `failed to connect to host=pg user=kusec: dial tcp 10.0.0.5:5432: connect: connection refused`

	cases := []struct {
		name          string
		authorization string
		usrAuth       *authMock
		keyAuth       *authMock
	}{
		{
			name:          "access-токен: база недоступна",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{err: fmt.Errorf("svc.Get: %w", errors.New(pgText))},
			keyAuth:       &authMock{},
		},
		{
			name:          "access-токен: сбой вместе с готовой сессией — сессия не используется",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{session: &sessionModel.Session{Id: 7, Admin: true}, err: errors.New(pgText)},
			keyAuth:       &authMock{},
		},
		{
			name:          "access-токен: в цепочке сбоя чужой код из internal/errs",
			authorization: "Bearer jwt-token",
			usrAuth:       &authMock{err: fmt.Errorf("svc.Get: %w", errs.ObjectNotFound)},
			keyAuth:       &authMock{},
		},
		{
			name:          "api-ключ: база недоступна",
			authorization: "Bearer " + constant.ApiKeyPrefix + "abc",
			usrAuth:       &authMock{},
			keyAuth:       &authMock{err: fmt.Errorf("svc.GetByKeyHash: %w", errors.New(pgText))},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			interceptor := GrpcInterceptorSession(sessionSvc, tc.keyAuth, tc.usrAuth)
			ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", tc.authorization))

			handlerCalled := false
			_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/kusec_v1.App/List"},
				func(context.Context, any) (any, error) {
					handlerCalled = true
					return nil, nil
				},
			)

			require.Error(t, err)
			assert.False(t, handlerCalled, "запрос не должен идти дальше как неавторизованный")
			// ровно service_not_available: ни текста сбоя, ни чужого кода
			assert.Equal(t, error(errs.ServiceNA), err)
			assert.NotErrorIs(t, err, errs.NotAuthorized)
		})
	}
}
