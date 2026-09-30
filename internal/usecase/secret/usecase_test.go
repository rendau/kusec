package secret

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	"github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
)

// ── Моки ────────────────────────────────────────────────

type svcMock struct {
	secret  *model.Main
	updated []*model.Edit
}

func (m *svcMock) List(_ context.Context, _ *model.ListReq) ([]*model.Main, int64, error) {
	return []*model.Main{m.secret}, 1, nil
}

func (m *svcMock) Get(_ context.Context, id string, _ bool) (*model.Main, bool, error) {
	if m.secret == nil || m.secret.Id != id {
		return nil, false, errs.ObjectNotFound
	}
	return m.secret, true, nil
}

func (m *svcMock) Create(_ context.Context, _ *model.Edit) (string, error) {
	return "", errs.NotImplemented
}

func (m *svcMock) Update(_ context.Context, _ string, obj *model.Edit) error {
	m.updated = append(m.updated, obj)
	return nil
}

func (m *svcMock) Delete(_ context.Context, _ string) error { return nil }

// itemSvcMock отдаёт item-ы с учётом фильтра Active — как реальный репозиторий.
type itemSvcMock struct {
	items []*itemModel.Main
}

func (m *itemSvcMock) List(_ context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
	result := make([]*itemModel.Main, 0, len(m.items))
	for _, item := range m.items {
		if pars.Active != nil && item.Active != *pars.Active {
			continue
		}
		result = append(result, item)
	}
	return result, int64(len(result)), nil
}

type txmStub struct{}

func (txmStub) TxFn(ctx context.Context, f func(context.Context) error) error { return f(ctx) }

type auditRecStub struct{}

func (auditRecStub) NewBatchId() string { return "batch" }

func (auditRecStub) RecordSecret(context.Context, *model.Main, *model.Main, string, *string) error {
	return nil
}

func (auditRecStub) RecordItem(context.Context, *itemModel.Main, *itemModel.Main, string, *string) error {
	return nil
}

// ── Тесты ───────────────────────────────────────────────

func newExactSlugFixture(items []*itemModel.Main, exactSlug bool) (*Usecase, *svcMock, *sessionService.Service) {
	svc := &svcMock{secret: &model.Main{Id: "sec1", AppId: "app1", Active: true, SlugName: "db", ExactSlug: exactSlug}}
	sessionSvc := sessionService.New("test-secret")
	uc := New(svc, nil, &itemSvcMock{items: items}, sessionSvc, txmStub{}, auditRecStub{})
	return uc, svc, sessionSvc
}

func TestUpdate_ExactSlug_BlockedByEmptyActiveItems(t *testing.T) {
	t.Parallel()

	uc, svc, sessionSvc := newExactSlugFixture([]*itemModel.Main{
		{Id: "i1", SecretId: "sec1", Active: true, Key: "PG_PASSWORD", Value: ""},
		{Id: "i2", SecretId: "sec1", Active: true, Key: "PG_USER", Value: "app"},
		{Id: "i3", SecretId: "sec1", Active: true, Key: "API_TOKEN", Value: ""},
	}, false)
	ctx := sessionSvc.WithContext(t.Context(), &sessionModel.Session{Id: 1, Admin: true})

	err := uc.Update(ctx, "sec1", &model.Edit{ExactSlug: new(true)})
	require.Error(t, err)

	full, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "expected errs.ErrFull, got %T: %v", err, err)
	assert.ErrorIs(t, full.Err, errs.InvalidRequest)
	assert.Contains(t, full.Desc, "exact_slug")
	assert.Contains(t, full.Desc, "API_TOKEN, PG_PASSWORD", "в описании — имена пустых ключей")
	assert.NotContains(t, full.Desc, "PG_USER", "заполненный ключ не упоминается")
	assert.NotContains(t, full.Desc, "app", "значения в ошибку не попадают")

	assert.Empty(t, svc.updated, "флаг не должен быть записан")
}

func TestUpdate_ExactSlug_AllowedWhenItemsFilledOrInactive(t *testing.T) {
	t.Parallel()

	uc, svc, sessionSvc := newExactSlugFixture([]*itemModel.Main{
		{Id: "i1", SecretId: "sec1", Active: true, Key: "PG_PASSWORD", Value: "filled"},
		// неактивный пустой item в кластер не уходит — не мешает
		{Id: "i2", SecretId: "sec1", Active: false, Key: "LEGACY", Value: ""},
	}, false)
	ctx := sessionSvc.WithContext(t.Context(), &sessionModel.Session{Id: 1, Admin: true})

	require.NoError(t, uc.Update(ctx, "sec1", &model.Edit{ExactSlug: new(true)}))
	require.Len(t, svc.updated, 1)
	assert.True(t, *svc.updated[0].ExactSlug)
}

func TestUpdate_ExactSlug_DisableNotGuarded(t *testing.T) {
	t.Parallel()

	// выключение флага при пустых item-ах разрешено: имя объекта меняется на
	// префиксное, живой объект не затрагивается
	uc, svc, sessionSvc := newExactSlugFixture([]*itemModel.Main{
		{Id: "i1", SecretId: "sec1", Active: true, Key: "PG_PASSWORD", Value: ""},
	}, true)
	ctx := sessionSvc.WithContext(t.Context(), &sessionModel.Session{Id: 1, Admin: true})

	require.NoError(t, uc.Update(ctx, "sec1", &model.Edit{ExactSlug: new(false)}))
	require.Len(t, svc.updated, 1)
}

func TestUpdate_ExactSlug_NonAdminNoPermission(t *testing.T) {
	t.Parallel()

	uc, svc, sessionSvc := newExactSlugFixture(nil, false)
	ctx := sessionSvc.WithContext(t.Context(), &sessionModel.Session{Id: 2, AppIds: []string{"app1"}})

	err := uc.Update(ctx, "sec1", &model.Edit{ExactSlug: new(true)})
	assert.ErrorIs(t, err, errs.NoPermission)
	assert.Empty(t, svc.updated)
}
