package apikey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

// ── Моки ────────────────────────────────────────────────

type svcMock struct {
	byHash  map[string]*model.Main
	byId    map[string]*model.Main
	created []*model.Edit
	updated []*model.Edit
	touched []string
}

func (m *svcMock) List(_ context.Context, _ *model.ListReq) ([]*model.Main, int64, error) {
	return nil, 0, nil
}

func (m *svcMock) Get(_ context.Context, id string, errNE bool) (*model.Main, bool, error) {
	item, ok := m.byId[id]
	if !ok && errNE {
		return nil, false, errs.ObjectNotFound
	}
	return item, ok, nil
}

func (m *svcMock) GetByKeyHash(_ context.Context, keyHash string) (*model.Main, bool, error) {
	item, ok := m.byHash[keyHash]
	return item, ok, nil
}

func (m *svcMock) Create(_ context.Context, obj *model.Edit) (string, error) {
	m.created = append(m.created, obj)
	return "new-id", nil
}

func (m *svcMock) Update(_ context.Context, _ string, obj *model.Edit) error {
	m.updated = append(m.updated, obj)
	return nil
}

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
	mcpOnlyKey, mcpOnlyHash, _, err := apikeyService.GenerateKey()
	require.NoError(t, err)

	svc := &svcMock{byHash: map[string]*model.Main{
		activeHash:   {Id: "k1", UsrId: 10, Active: true},
		inactiveHash: {Id: "k2", UsrId: 10, Active: false},
		orphanHash:   {Id: "k3", UsrId: 66, Active: true},
		mcpOnlyHash:  {Id: "k4", UsrId: 10, Active: true, McpOnly: true},
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

	// mcp_only-ключ: основной API отвергает, MCP-эндпоинт принимает
	_, err = u.SessionFromKey(context.Background(), mcpOnlyKey)
	assert.ErrorIs(t, err, errs.NotAuthorized)

	session, err = u.McpSessionFromKey(context.Background(), mcpOnlyKey)
	require.NoError(t, err)
	assert.Equal(t, int64(10), session.Id)

	// обычный ключ MCP-эндпоинт тоже принимает
	_, err = u.McpSessionFromKey(context.Background(), activeKey)
	require.NoError(t, err)
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

	id, key, err := u.Create(context.Background(), "мой ключ", nil, true)
	require.NoError(t, err)
	assert.Equal(t, "new-id", id)
	assert.NotEmpty(t, key)
	require.Len(t, svc.created, 1)
	assert.Equal(t, int64(10), *svc.created[0].UsrId)
	assert.True(t, *svc.created[0].McpOnly)
	// в БД уходит хэш, не сам ключ
	assert.Equal(t, apikeyService.HashKey(key), *svc.created[0].KeyHash)

	// не-админ не может выпустить ключ другому пользователю
	_, _, err = u.Create(context.Background(), "чужой", new(int64(20)), true)
	assert.ErrorIs(t, err, errs.NoPermission)

	// не-админ не может выпустить ключ с полным доступом к API
	_, _, err = u.Create(context.Background(), "полный", nil, false)
	assert.ErrorIs(t, err, errs.NoPermission)

	// админ — может
	uAdmin := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 1, Admin: true}})
	_, _, err = uAdmin.Create(context.Background(), "для сервисного", new(int64(20)), false)
	require.NoError(t, err)
	assert.Equal(t, int64(20), *svc.created[len(svc.created)-1].UsrId)
	assert.False(t, *svc.created[len(svc.created)-1].McpOnly)

	// неавторизованный
	uAnon := New(svc, usrSvc, &sessionSvcMock{})
	_, _, err = uAnon.Create(context.Background(), "x", nil, false)
	assert.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUpdate_McpOnlyPermissions(t *testing.T) {
	t.Parallel()

	usrSvc := &usrSvcMock{usrs: map[int64]*usrModel.Main{
		10: {Id: 10, Active: true},
	}}

	newSvc := func() *svcMock {
		return &svcMock{byId: map[string]*model.Main{
			"k-mcp":  {Id: "k-mcp", UsrId: 10, Active: true, McpOnly: true},
			"k-full": {Id: "k-full", UsrId: 10, Active: true, McpOnly: false},
		}}
	}

	// не-админ не может снять mcp_only со своего ключа
	svc := newSvc()
	u := New(svc, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 10}})
	err := u.Update(context.Background(), "k-mcp", nil, nil, new(false))
	assert.ErrorIs(t, err, errs.NoPermission)
	assert.Empty(t, svc.updated)

	// но может переименовать свой full-ключ, даже присылая mcp_only=false как есть
	err = u.Update(context.Background(), "k-full", nil, new("новое имя"), new(false))
	require.NoError(t, err)
	require.Len(t, svc.updated, 1)

	// и может ужесточить свой ключ до mcp_only
	err = u.Update(context.Background(), "k-full", nil, nil, new(true))
	require.NoError(t, err)

	// админ снимает mcp_only свободно
	svcAdmin := newSvc()
	uAdmin := New(svcAdmin, usrSvc, &sessionSvcMock{session: &sessionModel.Session{Id: 1, Admin: true}})
	err = uAdmin.Update(context.Background(), "k-mcp", nil, nil, new(false))
	require.NoError(t, err)
	require.Len(t, svcAdmin.updated, 1)
}
