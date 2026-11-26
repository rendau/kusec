package usr

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/domain/usr/model"
	usrService "github.com/rendau/kusec/internal/domain/usr/service"
	"github.com/rendau/kusec/internal/errs"
)

// ── Моки ────────────────────────────────────────────────

// repoMock — хранилище пользователей в памяти для настоящего доменного
// сервиса usr (логика TOTP проверяется вместе с ним).
type repoMock struct {
	items map[int64]*model.Main
}

func (r *repoMock) List(context.Context, *model.ListReq) ([]*model.Main, int64, error) {
	return nil, 0, errs.NotImplemented
}

func (r *repoMock) Get(_ context.Context, id int64) (*model.Main, bool, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, false, nil
	}
	cp := *item
	return &cp, true, nil
}

func (r *repoMock) GetByUsername(context.Context, string) (*model.Main, bool, error) {
	return nil, false, errs.NotImplemented
}

func (r *repoMock) HasAny(context.Context) (bool, error) { return true, nil }

func (r *repoMock) Create(context.Context, *model.Edit) (int64, error) {
	return 0, errs.NotImplemented
}

func (r *repoMock) Update(_ context.Context, id int64, obj *model.Edit) error {
	item := r.items[id]
	if obj.Active != nil {
		item.Active = *obj.Active
	}
	if obj.TotpEnabled != nil {
		item.TotpEnabled = *obj.TotpEnabled
	}
	if obj.TotpSecret != nil {
		item.TotpSecret = *obj.TotpSecret
	}
	return nil
}

func (r *repoMock) Delete(context.Context, int64) error { return nil }

type txmStub struct{}

func (txmStub) TxFn(ctx context.Context, f func(context.Context) error) error { return f(ctx) }

type auditRecStub struct{}

func (auditRecStub) RecordUsr(context.Context, *model.Main, *model.Main, *string) error { return nil }

func newFixture(t *testing.T, user *model.Main) (*Usecase, *repoMock, context.Context) {
	t.Helper()
	repo := &repoMock{items: map[int64]*model.Main{user.Id: user}}
	sessionSvc := sessionService.New("test-secret")
	uc := New(usrService.New(repo), sessionSvc, txmStub{}, auditRecStub{})
	// access-токен ещё жив, даже если пользователя уже выключили
	ctx := sessionSvc.WithContext(t.Context(), &sessionModel.Session{Id: user.Id})
	return uc, repo, ctx
}

func totpCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	require.NoError(t, err)
	return code
}

// ── Тесты ───────────────────────────────────────────────

// Выключенный, но не удалённый пользователь с живым access-токеном не может
// выпустить себе новые токены через 2FA.
func TestTotp_InactiveUserRejected(t *testing.T) {
	t.Parallel()

	uc, repo, ctx := newFixture(t, &model.Main{Id: 7, Username: "bob", Active: true})

	secret, _, err := uc.EnrollTotp(ctx)
	require.NoError(t, err)

	repo.items[7].Active = false // админ выключил пользователя

	_, err = uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.ErrorIs(t, err, errs.NotAuthorized)
	assert.False(t, repo.items[7].TotpEnabled)

	_, _, err = uc.EnrollTotp(ctx)
	require.ErrorIs(t, err, errs.NotAuthorized)

	require.ErrorIs(t, uc.DisableTotp(ctx, totpCode(t, secret)), errs.NotAuthorized)
}

// Подтверждение уже включённой 2FA не выдаёт новых токенов: иначе по одному
// коду из приложения можно продлевать сессию без конца.
func TestConfirmTotp_OnlyOnce(t *testing.T) {
	t.Parallel()

	uc, repo, ctx := newFixture(t, &model.Main{Id: 7, Username: "bob", Active: true})

	secret, _, err := uc.EnrollTotp(ctx)
	require.NoError(t, err)

	res, err := uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
	assert.True(t, repo.items[7].TotpEnabled)

	_, err = uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.ErrorIs(t, err, errs.TotpAlreadyOn)
}
