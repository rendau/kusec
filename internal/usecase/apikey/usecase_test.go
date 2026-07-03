package apikey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mechta-market/kusec/internal/domain/apikey/model"
	apikeyService "github.com/mechta-market/kusec/internal/domain/apikey/service"
	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	usrModel "github.com/mechta-market/kusec/internal/domain/usr/model"
	"github.com/mechta-market/kusec/internal/errs"
)

// ── Моки ────────────────────────────────────────────────

type svcMock struct {
	byHash  map[string]*model.Main
	created []*model.Edit
	touched []string
}

func (m *svcMock) List(_ context.Context, _ *model.ListReq) ([]*model.Main, int64, error) {
	return nil, 0, nil
}

func (m *svcMock) Get(_ context.Context, _ string, _ bool) (*model.Main, bool, error) {
	return nil, false, nil
}

func (m *svcMock) GetByKeyHash(_ context.Context, keyHash string) (*model.Main, bool, error) {
	item, ok := m.byHash[keyHash]
	return item, ok, nil
}

func (m *svcMock) Create(_ context.Context, obj *model.Edit) (string, error) {
	m.created = append(m.created, obj)
	return "new-id", nil
}

func (m *svcMock) Update(_ context.Context, _ string, _ *model.Edit) error { return nil }

func (m *svcMock) TouchLastUsed(_ context.Context, id string) error {
	m.touched = append(m.touched, id)
	return nil
}

func (m *svcMock) Delete(_ context.Context, _ string) error { return nil }

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

type sessionSvcMock struct {
	session *sessionModel.Session
}

func (m *sessionSvcMock) FromContext(_ context.Context) *sessionModel.Session { return m.session }
func (m *sessionSvcMock) CtxIsAuthorized(_ context.Context) bool              { return m.session.IsAuthorized() }
func (m *sessionSvcMock) CtxIsAdmin(_ context.Context) bool                   { return m.session.IsAdmin() }

// ── Тесты ───────────────────────────────────────────────

func TestSessionFromKey(t *testing.T) {
	t.Parallel()

	activeKey, activeHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	inactiveKey, inactiveHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	orphanKey, orphanHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	svc := &svcMock{byHash: map[string]*model.Main{
		activeHash:   {Id: "k1", UsrId: 10, Active: true},
		inactiveHash: {Id: "k2", UsrId: 10, Active: false},
		orphanHash:   {Id: "k3", UsrId: 66, Active: true},
	}}
	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true, IsAdmin: false, AppIds: []string{"app1"}},
	}}

	u := New(svc, usrSvc, &sessionSvcMock{})

	// валидный ключ активного пользователя
	session, err := u.SessionFromKey(context.Background(), activeKey)
	require.NoError(t, err)
	assert.Equal(t, int64(10), session.Id)
	assert.False(t, session.Admin)
	assert.Equal(t, []string{"app1"}, session.AppIds)
	assert.Equal(t, []string{"k1"}, svc.touched)

	// повторное использование в пределах минуты БД не трогает
	_, err = u.SessionFromKey(context.Background(), activeKey)
	require.NoError(t, err)
	assert.Len(t, svc.touched, 1)

	// неактивный ключ
	_, err = u.SessionFromKey(context.Background(), inactiveKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// владелец не найден
	_, err = u.SessionFromKey(context.Background(), orphanKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// не-ключ (JWT и мусор)
	_, err = u.SessionFromKey(context.Background(), "eyJhbGciOi...")
	assert.ErrorIs(t, err, errs.NotAuthorized)

	// валидный по формату, но неизвестный ключ
	unknownKey, _, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)
	_, err = u.SessionFromKey(context.Background(), unknownKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)
}

func TestCreate_Permissions(t *testing.T) {
	t.Parallel()

	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true},
		20: {Id: 20, Active: true},
	}}

	// не-админ создаёт ключ себе
	svc := &svcMock{}
	u := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 10}})

	id, key, err := u.Create(context.Background(), "мой ключ", nil)
	require.NoError(t, err)
	assert.Equal(t, "new-id", id)
	assert.NotEmpty(t, key)
	require.Len(t, svc.created, 1)
	assert.Equal(t, int64(10), *svc.created[0].UsrId)
	// в БД уходит хэш, не сам ключ
	assert.Equal(t, apikeyService.HashKey(key), *svc.created[0].KeyHash)

	// не-админ не может выпустить ключ другому пользователю
	_, _, err = u.Create(context.Background(), "чужой", new(int64(20)))
	assert.ErrorIs(t, err, errs.NoPermission)

	// админ — может
	uAdmin := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 1, Admin: true}})
	_, _, err = uAdmin.Create(context.Background(), "для сервисного", new(int64(20)))
	require.NoError(t, err)
	assert.Equal(t, int64(20), *svc.created[len(svc.created)-1].UsrId)

	// неавторизованный
	uAnon := New(svc, usrSvc, &sessionSvcMock{})
	_, _, err = uAnon.Create(context.Background(), "x", nil)
	assert.ErrorIs(t, err, errs.NotAuthorized)
}
