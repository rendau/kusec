package dashboard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

// ── Моки: сервисы отдают только числа ───────────────────

type appSvcMock struct{}

func (appSvcMock) List(context.Context, *appModel.ListReq) ([]*appModel.Main, int64, error) {
	return nil, 3, nil
}

func (appSvcMock) Get(context.Context, string, bool) (*appModel.Main, bool, error) {
	return nil, false, nil
}

type secretSvcMock struct{}

func (secretSvcMock) List(context.Context, *secretModel.ListReq) ([]*secretModel.Main, int64, error) {
	return []*secretModel.Main{{Id: "s1", AppId: "app1"}}, 4, nil
}

type itemSvcMock struct{}

func (itemSvcMock) List(context.Context, *itemModel.ListReq) ([]*itemModel.Main, int64, error) {
	return nil, 5, nil
}

type configMapSvcMock struct{}

func (configMapSvcMock) List(context.Context, *configmapModel.ListReq) ([]*configmapModel.Main, int64, error) {
	return []*configmapModel.Main{{Id: "c1", AppId: "app1"}}, 6, nil
}

type configItemSvcMock struct{}

func (configItemSvcMock) List(context.Context, *configitemModel.ListReq) ([]*configitemModel.Main, int64, error) {
	return nil, 7, nil
}

// usrSvcMock считает обращения: не-админу число пользователей не только не
// показывается, но и не запрашивается.
type usrSvcMock struct {
	calls int
}

func (m *usrSvcMock) List(_ context.Context, pars *usrModel.ListReq) ([]*usrModel.Main, int64, error) {
	m.calls++
	if pars.Active != nil {
		return nil, 8, nil
	}
	return nil, 9, nil
}

func getSummary(t *testing.T, session *sessionModel.Session) (*Summary, *usrSvcMock, error) {
	t.Helper()

	sessionSvc := sessionService.New("test-secret")
	usrSvc := &usrSvcMock{}
	usecase := New(appSvcMock{}, secretSvcMock{}, itemSvcMock{}, configMapSvcMock{}, configItemSvcMock{}, usrSvc, sessionSvc)

	summary, err := usecase.Get(sessionSvc.WithContext(context.Background(), session))
	return summary, usrSvc, err
}

// Число пользователей на дашборде — только админу: список пользователей
// остальным закрыт, и размер команды им тоже не показывается.
func TestGet_UsrCountAdminOnly(t *testing.T) {
	t.Parallel()

	summary, usrSvc, err := getSummary(t, &sessionModel.Session{Id: 1, Admin: true})
	require.NoError(t, err)
	assert.Equal(t, Count{Total: 9, Active: 8}, summary.Usr)
	assert.Equal(t, 2, usrSvc.calls)
	assert.Equal(t, Count{Total: 3, Active: 3}, summary.App)

	for name, session := range map[string]*sessionModel.Session{
		"не-админ с all_apps":            {Id: 2, AllApps: true},
		"не-админ со списком приложений": {Id: 3, AppIds: []string{"app1"}},
		"не-админ без приложений":        {Id: 4},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			summary, usrSvc, err := getSummary(t, session)
			require.NoError(t, err)
			assert.Equal(t, Count{}, summary.Usr, "число пользователей не-админу не показывается")
			assert.Zero(t, usrSvc.calls)
		})
	}

	// остальные числа не-админу с доступом к приложениям отдаются как раньше
	summary, _, err = getSummary(t, &sessionModel.Session{Id: 2, AllApps: true})
	require.NoError(t, err)
	assert.Equal(t, Count{Total: 3, Active: 3}, summary.App)
	assert.Equal(t, Count{Total: 4, Active: 4}, summary.Secret)
	assert.Equal(t, Count{Total: 5, Active: 5}, summary.Item)

	// без входа — not_authorized
	_, _, err = getSummary(t, nil)
	require.ErrorIs(t, err, errs.NotAuthorized)
}
