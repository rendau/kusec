package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/errs"
	kubeService "github.com/rendau/kusec/internal/service/kube"
)

// kubeSvcMock запоминает область, с которой вызван sync. Остальные методы
// интерфейса в этих тестах не вызываются.
type kubeSvcMock struct {
	KubeServiceI

	calls     int
	gotAppIds []string
	gotAdopt  bool
}

func (m *kubeSvcMock) record(appIds []string, adoptForeign bool) {
	m.calls++
	m.gotAppIds = appIds
	m.gotAdopt = adoptForeign
}

func (m *kubeSvcMock) SyncSecrets(_ context.Context, appIds []string, adoptForeign bool) (*kubeService.SyncResult, error) {
	m.record(appIds, adoptForeign)
	return &kubeService.SyncResult{}, nil
}

func (m *kubeSvcMock) SyncConfigMaps(_ context.Context, appIds []string, adoptForeign bool) (*kubeService.SyncResult, error) {
	m.record(appIds, adoptForeign)
	return &kubeService.SyncResult{}, nil
}

func (m *kubeSvcMock) Sync(_ context.Context, appIds []string, adoptForeign bool) (*kubeService.SyncResult, *kubeService.SyncResult, error) {
	m.record(appIds, adoptForeign)
	return &kubeService.SyncResult{}, &kubeService.SyncResult{}, nil
}

// Область sync. Главное: пользователь без единого приложения не запускает
// sync вовсе — пустой список kube-сервис понял бы как «все приложения».
func TestSyncScope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		session *sessionModel.Session
		appId   *string
		wantErr error
		// область, переданная в kube-сервис; nil — все приложения
		wantAppIds []string
	}{
		{
			name:    "без приложений, sync всего — отказ",
			session: &sessionModel.Session{Id: 2},
			wantErr: errs.NoPermission,
		},
		{
			name:    "без приложений (пустой список), sync всего — отказ",
			session: &sessionModel.Session{Id: 2, AppIds: []string{}},
			wantErr: errs.NoPermission,
		},
		{
			name:    "без приложений, пустой app_id — отказ",
			session: &sessionModel.Session{Id: 2},
			appId:   new(""),
			wantErr: errs.NoPermission,
		},
		{
			name:    "без приложений, sync чужого — отказ",
			session: &sessionModel.Session{Id: 2},
			appId:   new("app1"),
			wantErr: errs.NoPermission,
		},
		{
			name:       "свои приложения, sync всего — только свои",
			session:    &sessionModel.Session{Id: 2, AppIds: []string{"app1", "app2"}},
			wantAppIds: []string{"app1", "app2"},
		},
		{
			name:       "своё приложение",
			session:    &sessionModel.Session{Id: 2, AppIds: []string{"app1", "app2"}},
			appId:      new("app2"),
			wantAppIds: []string{"app2"},
		},
		{
			name:    "чужое приложение — отказ",
			session: &sessionModel.Session{Id: 2, AppIds: []string{"app1"}},
			appId:   new("app9"),
			wantErr: errs.NoPermission,
		},
		{
			name:       "all_apps, sync всего — все",
			session:    &sessionModel.Session{Id: 2, AllApps: true},
			wantAppIds: nil,
		},
		{
			name:       "all_apps, одно приложение",
			session:    &sessionModel.Session{Id: 2, AllApps: true},
			appId:      new("app9"),
			wantAppIds: []string{"app9"},
		},
		{
			name:       "админ, sync всего — все",
			session:    &sessionModel.Session{Id: 1, Admin: true},
			wantAppIds: nil,
		},
		{
			name:    "без входа — not_authorized",
			session: nil,
			wantErr: errs.NotAuthorized,
		},
	}

	sessionSvc := sessionService.New("test-secret")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := sessionSvc.WithContext(t.Context(), tc.session)

			runs := map[string]func(uc *Usecase) error{
				"Sync": func(uc *Usecase) error {
					_, _, err := uc.Sync(ctx, tc.appId)
					return err
				},
				"SyncSecrets": func(uc *Usecase) error {
					_, err := uc.SyncSecrets(ctx, tc.appId)
					return err
				},
				"SyncConfigMaps": func(uc *Usecase) error {
					_, err := uc.SyncConfigMaps(ctx, tc.appId)
					return err
				},
			}
			for method, run := range runs {
				svc := &kubeSvcMock{}
				uc := New(svc, nil, nil, nil, sessionSvc)

				err := run(uc)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr, method)
					assert.Zero(t, svc.calls, "%s: kube-сервис не должен вызываться", method)
					continue
				}
				require.NoError(t, err, method)
				require.Equal(t, 1, svc.calls, method)
				assert.Equal(t, tc.wantAppIds, svc.gotAppIds, method)
				// чужой объект кластера усыновляет только sync админа
				assert.Equal(t, tc.session.IsAdmin(), svc.gotAdopt, method)
			}
		})
	}
}
