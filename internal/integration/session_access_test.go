package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/constant"
	apikeyModel "github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyDb "github.com/rendau/kusec/internal/domain/apikey/repo/db"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	appModel "github.com/rendau/kusec/internal/domain/app/model"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	syncrunDb "github.com/rendau/kusec/internal/domain/syncrun/repo/db"
	syncrunService "github.com/rendau/kusec/internal/domain/syncrun/service"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	usrDb "github.com/rendau/kusec/internal/domain/usr/repo/db"
	usrService "github.com/rendau/kusec/internal/domain/usr/service"
	"github.com/rendau/kusec/internal/errs"
	cascadeService "github.com/rendau/kusec/internal/service/cascade"
	kubeService "github.com/rendau/kusec/internal/service/kube"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	auditUsc "github.com/rendau/kusec/internal/usecase/audit"
	configitemUsc "github.com/rendau/kusec/internal/usecase/configitem"
	configmapUsc "github.com/rendau/kusec/internal/usecase/configmap"
	dashboardUsc "github.com/rendau/kusec/internal/usecase/dashboard"
	kubeUsc "github.com/rendau/kusec/internal/usecase/kube"
	monitoringUsc "github.com/rendau/kusec/internal/usecase/monitoring"
	secretUsc "github.com/rendau/kusec/internal/usecase/secret"
	syncrunUsc "github.com/rendau/kusec/internal/usecase/syncrun"
	transferUsc "github.com/rendau/kusec/internal/usecase/transfer"
	usrUsc "github.com/rendau/kusec/internal/usecase/usr"
)

const accessPassword = "Access-Passw0rd!"

// accessStack — stack плюс все usecase-ы, которые читают данные по правам
// сессии: на них проверяется, что пользователь видит ровно свои приложения.
type accessStack struct {
	*stack

	// suffix делает логины и slug-и уникальными: тесты можно гонять
	// повторно на той же базе.
	suffix string

	usrSvc     *usrService.Service
	syncRunSvc *syncrunService.Service

	secretUsecase     *secretUsc.Usecase
	configMapUsecase  *configmapUsc.Usecase
	configItemUsecase *configitemUsc.Usecase
	auditUsecase      *auditUsc.Usecase
	syncRunUsecase    *syncrunUsc.Usecase
	dashboardUsecase  *dashboardUsc.Usecase
	transferUsecase   *transferUsc.Usecase
	monitoringUsecase *monitoringUsc.Usecase
	kubeUsecase       *kubeUsc.Usecase
	apikeyUsecase     *apikeyUsc.Usecase
}

func newAccessStack(t *testing.T) *accessStack {
	t.Helper()

	s := newStack(t)
	a := &accessStack{
		stack:  s,
		suffix: fmt.Sprintf("%x", time.Now().UnixNano()),
	}

	a.usrSvc = usrService.New(usrDb.New(s.pool))
	a.syncRunSvc = syncrunService.New(syncrunDb.New(s.pool))
	apikeySvc := apikeyService.New(apikeyDb.New(s.pool))

	cascadeSvc := cascadeService.New(s.appSvc, s.secretSvc, s.itemSvc, s.configMapSvc, s.configItemSvc, s.auditRec)
	kubeSvc := kubeService.New(
		s.appSvc, s.secretSvc, s.itemSvc, s.configMapSvc, s.configItemSvc, s.txm, s.auditRec, a.syncRunSvc,
	)

	a.secretUsecase = secretUsc.New(s.secretSvc, s.appSvc, s.itemSvc, cascadeSvc, s.sessionSvc, s.txm, s.auditRec)
	a.configMapUsecase = configmapUsc.New(s.configMapSvc, s.appSvc, cascadeSvc, s.sessionSvc, s.txm, s.auditRec)
	a.configItemUsecase = configitemUsc.New(s.configItemSvc, s.configMapSvc, s.sessionSvc, s.txm, s.auditRec)
	a.auditUsecase = auditUsc.New(s.auditSvc, s.sessionSvc)
	a.syncRunUsecase = syncrunUsc.New(a.syncRunSvc, s.appSvc, s.sessionSvc)
	a.dashboardUsecase = dashboardUsc.New(
		s.appSvc, s.secretSvc, s.itemSvc, s.configMapSvc, s.configItemSvc, a.usrSvc, s.sessionSvc,
	)
	a.transferUsecase = transferUsc.New(
		s.appSvc, s.secretSvc, s.itemSvc, s.configMapSvc, s.configItemSvc, s.sessionSvc,
	)
	a.monitoringUsecase = monitoringUsc.New(
		s.appSvc, s.secretSvc, s.itemSvc, s.configMapSvc, s.configItemSvc, s.auditSvc, kubeSvc, s.sessionSvc,
	)
	a.kubeUsecase = kubeUsc.New(kubeSvc, s.appSvc, s.secretSvc, s.configMapSvc, s.sessionSvc)
	a.apikeyUsecase = apikeyUsc.New(apikeySvc, a.usrSvc, s.sessionSvc, s.txm, s.auditRec, s.usrUsecase)

	return a
}

// name — уникальное для запуска имя (логин, slug).
func (a *accessStack) name(base string) string {
	return base + "-" + a.suffix
}

// newUser создаёт пользователя с паролем accessPassword и возвращает его id.
func (a *accessStack) newUser(t *testing.T, base string, edit usrModel.Edit) (int64, string) {
	t.Helper()
	username := a.name(base)
	edit.Name = new(username)
	edit.Username = new(username)
	edit.Password = new(accessPassword)
	id, err := a.usrUsecase.Create(a.adminCtx(), &edit)
	require.NoError(t, err)
	return id, username
}

// tokenCtx — контекст запроса с access-токеном: сессию строит usr usecase по
// живой записи пользователя, как это делает session-интерсептор.
func (a *accessStack) tokenCtx(t *testing.T, token string) context.Context {
	t.Helper()
	session, err := a.usrUsecase.SessionFromToken(context.Background(), token)
	require.NoError(t, err)
	return a.sessionSvc.WithContext(context.Background(), session)
}

func (a *accessStack) login(t *testing.T, username, password string) *usrUsc.LoginResult {
	t.Helper()
	res, err := a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{
		Username: username,
		Password: password,
		ClientIp: "198.51.100.7",
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Jwt)
	return res
}

// loginCtx — вход и контекст запроса вошедшего пользователя.
func (a *accessStack) loginCtx(t *testing.T, username string) context.Context {
	t.Helper()
	return a.tokenCtx(t, a.login(t, username, accessPassword).Jwt)
}

// appData — приложение со всеми видами дочерних записей.
type appData struct {
	appId, appSlug string
	secretId       string
	itemId         string
	configMapId    string
	configItemId   string
}

// newAppData создаёт приложение с секретом, ключом, configmap-ом и его
// ключом. Дочерние записи — через usecase-ы под админом: так появляются и
// записи аудита этого приложения.
func (a *accessStack) newAppData(t *testing.T, base string) appData {
	t.Helper()
	ctx := a.adminCtx()

	d := appData{appSlug: a.name(base)}
	d.appId, d.secretId = a.newApp(t, d.appSlug)

	var err error
	d.itemId, err = a.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId: &d.secretId, Active: new(true), Key: new("TOKEN"), Value: new("secret-value-" + base),
	})
	require.NoError(t, err)

	d.configMapId, err = a.configMapUsecase.Create(ctx, &configmapModel.Edit{
		AppId: &d.appId, Active: new(true), SlugName: new("config"),
	})
	require.NoError(t, err)

	d.configItemId, err = a.configItemUsecase.Create(ctx, &configitemModel.Edit{
		ConfigMapId: &d.configMapId, Active: new(true), Key: new("HTTP_PORT"), Value: new("8080"),
	})
	require.NoError(t, err)

	return d
}

// newSyncRun создаёт запись журнала sync: appId nil — общий запуск.
func (a *accessStack) newSyncRun(t *testing.T, appId *string) string {
	t.Helper()
	ctx := context.Background()

	runId, err := a.syncRunSvc.Start(ctx, &syncrunModel.Main{
		ActorName: "Integration Tester",
		Source:    constant.SourceApi,
		AppId:     appId,
	})
	require.NoError(t, err)

	require.NoError(t, a.syncRunSvc.AddObjects(ctx, runId, []*syncrunModel.Object{{
		Namespace: "itest", KubeKind: "Secret", KubeName: "kusec-any-main", Op: syncrunModel.OpCreated,
	}}))

	return runId
}

func pageOf(size int64) commonModel.ListParams {
	return commonModel.ListParams{PageSize: size, WithTotalCount: true}
}

// requireNothingVisible — сессия не видит ни одного приложения и ничего в
// них: все списки пусты, прямые обращения и sync — отказ.
func (a *accessStack) requireNothingVisible(t *testing.T, ctx context.Context, apps ...appData) {
	t.Helper()

	// списки без фильтра по приложению — пустые (а не «всё подряд»)
	appItems, total, err := a.appUsecase.List(ctx, &appModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	assert.Empty(t, appItems, "app list")
	assert.Zero(t, total)

	secrets, total, err := a.secretUsecase.List(ctx, &secretModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	assert.Empty(t, secrets, "secret list")
	assert.Zero(t, total)

	configMaps, total, err := a.configMapUsecase.List(ctx, &configmapModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	assert.Empty(t, configMaps, "configmap list")
	assert.Zero(t, total)

	auditItems, total, err := a.auditUsecase.List(ctx, &auditModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	assert.Empty(t, auditItems, "audit list")
	assert.Zero(t, total)

	runs, total, err := a.syncRunUsecase.List(ctx, &syncrunModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	assert.Empty(t, runs, "sync run list")
	assert.Zero(t, total)

	// ключи без указания секрета/configmap-а не-админу не отдаются вовсе
	_, _, err = a.itemUsecase.List(ctx, &itemModel.ListReq{ListParams: pageOf(100)})
	require.ErrorIs(t, err, errs.NoPermission, "item list")
	_, _, err = a.configItemUsecase.List(ctx, &configitemModel.ListReq{ListParams: pageOf(100)})
	require.ErrorIs(t, err, errs.NoPermission, "config item list")

	tree, err := a.transferUsecase.Tree(ctx)
	require.NoError(t, err)
	assert.Empty(t, tree, "transfer tree")

	summary, err := a.dashboardUsecase.Get(ctx)
	require.NoError(t, err)
	assert.Zero(t, summary.App, "dashboard app")
	assert.Zero(t, summary.Secret, "dashboard secret")
	assert.Zero(t, summary.Item, "dashboard item")
	assert.Zero(t, summary.ConfigMap, "dashboard configmap")
	assert.Zero(t, summary.ConfigItem, "dashboard config item")
	assert.Empty(t, summary.RecentSecrets, "dashboard recent secrets")
	assert.Zero(t, summary.Usr, "dashboard usr: число пользователей — только админу")

	// sync «всего» — отказ: пустой список приложений не должен стать «все»
	_, _, err = a.kubeUsecase.Sync(ctx, nil)
	require.ErrorIs(t, err, errs.NoPermission, "sync all")
	_, err = a.kubeUsecase.SyncSecrets(ctx, nil)
	require.ErrorIs(t, err, errs.NoPermission, "sync secrets all")
	_, err = a.kubeUsecase.SyncConfigMaps(ctx, nil)
	require.ErrorIs(t, err, errs.NoPermission, "sync configmaps all")

	for _, d := range apps {
		a.requireAppHidden(t, ctx, d)
	}
}

// requireAppHidden — приложение d и всё в нём сессии недоступно.
func (a *accessStack) requireAppHidden(t *testing.T, ctx context.Context, d appData) {
	t.Helper()

	_, err := a.appUsecase.Get(ctx, d.appId)
	require.ErrorIs(t, err, errs.NoPermission, "app get")

	// поиск по slug чужого приложения ничего не находит
	appItems, _, err := a.appUsecase.List(ctx, &appModel.ListReq{ListParams: pageOf(100), Search: &d.appSlug})
	require.NoError(t, err)
	assert.Empty(t, appItems, "app search")
	appItems, _, err = a.appUsecase.List(ctx, &appModel.ListReq{ListParams: pageOf(100), Ids: []string{d.appId}})
	require.NoError(t, err)
	assert.Empty(t, appItems, "app list by id")

	_, _, err = a.secretUsecase.List(ctx, &secretModel.ListReq{AppId: &d.appId})
	require.ErrorIs(t, err, errs.NoPermission, "secret list by app")
	_, err = a.secretUsecase.Get(ctx, d.secretId)
	require.ErrorIs(t, err, errs.NoPermission, "secret get")

	_, _, err = a.configMapUsecase.List(ctx, &configmapModel.ListReq{AppId: &d.appId})
	require.ErrorIs(t, err, errs.NoPermission, "configmap list by app")
	_, err = a.configMapUsecase.Get(ctx, d.configMapId)
	require.ErrorIs(t, err, errs.NoPermission, "configmap get")

	_, _, err = a.itemUsecase.List(ctx, &itemModel.ListReq{SecretId: &d.secretId})
	require.ErrorIs(t, err, errs.NoPermission, "item list by secret")
	_, _, err = a.itemUsecase.List(ctx, &itemModel.ListReq{SecretIds: []string{d.secretId}})
	require.ErrorIs(t, err, errs.NoPermission, "item list by secrets")
	_, _, err = a.itemUsecase.List(ctx, &itemModel.ListReq{AppId: &d.appId})
	require.ErrorIs(t, err, errs.NoPermission, "item list by app")
	_, err = a.itemUsecase.Get(ctx, d.itemId)
	require.ErrorIs(t, err, errs.NoPermission, "item get")

	_, _, err = a.configItemUsecase.List(ctx, &configitemModel.ListReq{ConfigMapId: &d.configMapId})
	require.ErrorIs(t, err, errs.NoPermission, "config item list by configmap")
	_, _, err = a.configItemUsecase.List(ctx, &configitemModel.ListReq{ConfigMapIds: []string{d.configMapId}})
	require.ErrorIs(t, err, errs.NoPermission, "config item list by configmaps")
	_, _, err = a.configItemUsecase.List(ctx, &configitemModel.ListReq{AppId: &d.appId})
	require.ErrorIs(t, err, errs.NoPermission, "config item list by app")
	_, err = a.configItemUsecase.Get(ctx, d.configItemId)
	require.ErrorIs(t, err, errs.NoPermission, "config item get")

	_, _, err = a.auditUsecase.List(ctx, &auditModel.ListReq{ListParams: pageOf(100), AppId: &d.appId})
	require.ErrorIs(t, err, errs.NoPermission, "audit list by app")
	_, _, err = a.syncRunUsecase.List(ctx, &syncrunModel.ListReq{ListParams: pageOf(100), AppId: &d.appId})
	require.ErrorIs(t, err, errs.NoPermission, "sync run list by app")

	_, err = a.monitoringUsecase.Keys(ctx, d.appId)
	require.ErrorIs(t, err, errs.NoPermission, "monitoring keys")
	_, _, err = a.monitoringUsecase.Drift(ctx, d.appId)
	require.ErrorIs(t, err, errs.NoPermission, "monitoring drift")
	resolved, err := a.monitoringUsecase.Resolve(ctx, "itest", kubeService.SecretName(d.appSlug, "main", false))
	require.NoError(t, err)
	assert.Nil(t, resolved, "monitoring resolve")

	// sync чужого приложения и чтение его объектов из кластера
	_, _, err = a.kubeUsecase.Sync(ctx, &d.appId)
	require.ErrorIs(t, err, errs.NoPermission, "sync app")
	_, err = a.kubeUsecase.SyncSecrets(ctx, &d.appId)
	require.ErrorIs(t, err, errs.NoPermission, "sync secrets app")
	_, err = a.kubeUsecase.SyncConfigMaps(ctx, &d.appId)
	require.ErrorIs(t, err, errs.NoPermission, "sync configmaps app")
	_, _, _, err = a.kubeUsecase.GetClusterSecret(ctx, d.secretId)
	require.ErrorIs(t, err, errs.NoPermission, "cluster secret")
	_, _, _, err = a.kubeUsecase.GetClusterConfigMap(ctx, d.configMapId)
	require.ErrorIs(t, err, errs.NoPermission, "cluster configmap")

	// и никаких изменений
	_, err = a.secretUsecase.Create(ctx, &secretModel.Edit{AppId: &d.appId, Active: new(true), SlugName: new("intruder")})
	require.ErrorIs(t, err, errs.NoPermission, "secret create")
	_, err = a.itemUsecase.Create(ctx, &itemModel.Edit{SecretId: &d.secretId, Active: new(true), Key: new("X"), Value: new("v")})
	require.ErrorIs(t, err, errs.NoPermission, "item create")
	require.ErrorIs(t, a.itemUsecase.Update(ctx, d.itemId, &itemModel.Edit{Value: new("v")}), errs.NoPermission, "item update")
	require.ErrorIs(t, a.itemUsecase.Delete(ctx, d.itemId), errs.NoPermission, "item delete")
	require.ErrorIs(t, a.secretUsecase.Delete(ctx, d.secretId), errs.NoPermission, "secret delete")
	require.ErrorIs(t, a.configMapUsecase.Delete(ctx, d.configMapId), errs.NoPermission, "configmap delete")
}

// requireAppVisible — приложение d и всё в нём сессии доступно. Sync вне
// кластера заканчивается not_in_cluster: значит, проверку доступа он прошёл.
func (a *accessStack) requireAppVisible(t *testing.T, ctx context.Context, d appData) {
	t.Helper()

	app, err := a.appUsecase.Get(ctx, d.appId)
	require.NoError(t, err)
	assert.Equal(t, d.appSlug, app.SlugName)

	appItems, _, err := a.appUsecase.List(ctx, &appModel.ListReq{ListParams: pageOf(100), Ids: []string{d.appId}})
	require.NoError(t, err)
	require.Len(t, appItems, 1)

	secrets, _, err := a.secretUsecase.List(ctx, &secretModel.ListReq{AppId: &d.appId})
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	assert.Equal(t, d.secretId, secrets[0].Id)

	configMaps, _, err := a.configMapUsecase.List(ctx, &configmapModel.ListReq{AppId: &d.appId})
	require.NoError(t, err)
	require.Len(t, configMaps, 1)
	assert.Equal(t, d.configMapId, configMaps[0].Id)

	items, _, err := a.itemUsecase.List(ctx, &itemModel.ListReq{SecretId: &d.secretId})
	require.NoError(t, err)
	require.Len(t, items, 1)

	configItems, _, err := a.configItemUsecase.List(ctx, &configitemModel.ListReq{ConfigMapId: &d.configMapId})
	require.NoError(t, err)
	require.Len(t, configItems, 1)

	auditItems, _, err := a.auditUsecase.List(ctx, &auditModel.ListReq{ListParams: pageOf(100), AppId: &d.appId})
	require.NoError(t, err)
	assert.NotEmpty(t, auditItems, "audit list by app")

	keys, err := a.monitoringUsecase.Keys(ctx, d.appId)
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	resolved, err := a.monitoringUsecase.Resolve(ctx, "itest", kubeService.SecretName(d.appSlug, "main", false))
	require.NoError(t, err)
	require.NotNil(t, resolved)
	assert.Equal(t, d.secretId, resolved.ObjectId)

	_, _, err = a.kubeUsecase.Sync(ctx, &d.appId)
	require.ErrorIs(t, err, errs.NotInCluster, "sync app")
}

// treeAppIds — id приложений в дереве transfer.
func treeAppIds(tree []*transferUsc.TreeApp) []string {
	return lo.Map(tree, func(v *transferUsc.TreeApp, _ int) string { return v.Id })
}

// ── Доступ к приложениям ────────────────────────────────

// Пользователь без приложений (пустой app_ids, all_apps=false) не видит
// ничего ни в одном списке и не может запустить sync — ни общий, ни чужого
// приложения. Раньше пустой список значил «доступ ко всему».
func TestUserWithoutAppsSeesNothing(t *testing.T) {
	a := newAccessStack(t)

	appA := a.newAppData(t, "noapps-a")
	appB := a.newAppData(t, "noapps-b")
	runA := a.newSyncRun(t, &appA.appId)
	runGlobal := a.newSyncRun(t, nil)

	_, username := a.newUser(t, "noapps-user", usrModel.Edit{AppIds: []string{}})
	ctx := a.loginCtx(t, username)

	// сессия построена по записи: ни админа, ни all_apps, ни приложений
	session := a.sessionSvc.FromContext(ctx)
	require.True(t, session.IsAuthorized())
	require.False(t, session.Admin)
	require.False(t, session.AllApps)
	require.Empty(t, session.AppIds)

	a.requireNothingVisible(t, ctx, appA, appB)

	// журнал sync: ни запуск чужого приложения, ни общий
	_, err := a.syncRunUsecase.Get(ctx, runA)
	require.ErrorIs(t, err, errs.NoPermission)
	_, err = a.syncRunUsecase.Get(ctx, runGlobal)
	require.ErrorIs(t, err, errs.NoPermission)

	// то же — по API-ключу такого пользователя (mcp_only он выпускает себе сам)
	_, key, err := a.apikeyUsecase.Create(ctx, "own key", nil, constant.ApiKeyScopeMcpOnly, accessPassword)
	require.NoError(t, err)
	keySession, err := a.apikeyUsecase.McpSessionFromKey(context.Background(), key)
	require.NoError(t, err)
	a.requireNothingVisible(t, a.sessionSvc.WithContext(context.Background(), keySession), appA, appB)
}

// Пользователь со списком приложений видит ровно их; общий sync идёт только
// по ним, а объекты общего запуска sync показываются только из его namespace-ов.
func TestUserWithAppListSeesOnlyListed(t *testing.T) {
	a := newAccessStack(t)

	appA := a.newAppData(t, "listed-a")
	appB := a.newAppData(t, "listed-b")
	runA := a.newSyncRun(t, &appA.appId)
	runB := a.newSyncRun(t, &appB.appId)

	_, username := a.newUser(t, "listed-user", usrModel.Edit{AppIds: []string{appA.appId}})
	ctx := a.loginCtx(t, username)

	a.requireAppVisible(t, ctx, appA)
	a.requireAppHidden(t, ctx, appB)

	appItems, total, err := a.appUsecase.List(ctx, &appModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	require.Len(t, appItems, 1)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, appA.appId, appItems[0].Id)

	secrets, _, err := a.secretUsecase.List(ctx, &secretModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	assert.Equal(t, appA.secretId, secrets[0].Id)

	configMaps, _, err := a.configMapUsecase.List(ctx, &configmapModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	require.Len(t, configMaps, 1)
	assert.Equal(t, appA.configMapId, configMaps[0].Id)

	tree, err := a.transferUsecase.Tree(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{appA.appId}, treeAppIds(tree))

	summary, err := a.dashboardUsecase.Get(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, summary.App.Total)
	assert.EqualValues(t, 1, summary.Secret.Total)
	assert.EqualValues(t, 1, summary.Item.Total)
	assert.EqualValues(t, 1, summary.ConfigMap.Total)
	assert.EqualValues(t, 1, summary.ConfigItem.Total)
	assert.Zero(t, summary.Usr, "число пользователей — только админу")

	auditItems, _, err := a.auditUsecase.List(ctx, &auditModel.ListReq{ListParams: pageOf(100)})
	require.NoError(t, err)
	require.NotEmpty(t, auditItems)
	for _, item := range auditItems {
		require.NotNil(t, item.AppId)
		assert.Equal(t, appA.appId, *item.AppId)
	}

	// журнал sync: свой запуск виден, чужой нет
	run, err := a.syncRunUsecase.Get(ctx, runA)
	require.NoError(t, err)
	assert.Len(t, run.Objects, 1)
	_, err = a.syncRunUsecase.Get(ctx, runB)
	require.ErrorIs(t, err, errs.NoPermission)

	// общий sync — в пределах своих приложений (вне кластера — not_in_cluster)
	_, _, err = a.kubeUsecase.Sync(ctx, nil)
	require.ErrorIs(t, err, errs.NotInCluster)
}

// all_apps даёт не-админу все приложения, но не права администратора.
func TestAllAppsUserSeesEverything(t *testing.T) {
	a := newAccessStack(t)

	appA := a.newAppData(t, "allapps-a")
	appB := a.newAppData(t, "allapps-b")
	runA := a.newSyncRun(t, &appA.appId)
	runGlobal := a.newSyncRun(t, nil)

	usrId, username := a.newUser(t, "allapps-user", usrModel.Edit{AllApps: new(true)})
	token := a.login(t, username, accessPassword).Jwt
	ctx := a.tokenCtx(t, token)

	session := a.sessionSvc.FromContext(ctx)
	require.True(t, session.AllApps)
	require.False(t, session.Admin)

	a.requireAppVisible(t, ctx, appA)
	a.requireAppVisible(t, ctx, appB)

	appItems, _, err := a.appUsecase.List(ctx, &appModel.ListReq{ListParams: pageOf(100), Search: &a.suffix})
	require.NoError(t, err)
	assert.ElementsMatch(t,
		[]string{appA.appId, appB.appId},
		lo.Map(appItems, func(v *appModel.Main, _ int) string { return v.Id }),
	)

	tree, err := a.transferUsecase.Tree(ctx)
	require.NoError(t, err)
	assert.Subset(t, treeAppIds(tree), []string{appA.appId, appB.appId})

	summary, err := a.dashboardUsecase.Get(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, summary.App.Total, int64(2))
	assert.GreaterOrEqual(t, summary.Secret.Total, int64(2))
	assert.Zero(t, summary.Usr, "число пользователей — только админу")

	adminSummary, err := a.dashboardUsecase.Get(a.adminCtx())
	require.NoError(t, err)
	assert.Positive(t, adminSummary.Usr.Total)

	// аудит: записи всех приложений, но не весь аудит — записи без
	// приложения (учётки, ключи, общий sync) видит только админ
	_, key, err := a.apikeyUsecase.Create(ctx, "allapps key", nil, constant.ApiKeyScopeMcpOnly, accessPassword)
	require.NoError(t, err)
	mcpSession, err := a.apikeyUsecase.McpSessionFromKey(context.Background(), key)
	require.NoError(t, err)

	for name, auditCtx := range map[string]context.Context{
		"вход в админку": ctx,
		"mcp-ключ":       a.sessionSvc.WithContext(context.Background(), mcpSession),
	} {
		auditItems, total, err := a.auditUsecase.List(auditCtx, &auditModel.ListReq{ListParams: pageOf(100)})
		require.NoError(t, err, name)
		require.NotEmpty(t, auditItems, name)
		assert.Positive(t, total, name)
		for _, item := range auditItems {
			require.NotNil(t, item.AppId, "%s: запись %d (%s) без приложения", name, item.Id, item.EntityType)
		}

		for _, entityType := range []string{auditModel.EntityUsr, auditModel.EntityApiKey} {
			auditItems, total, err = a.auditUsecase.List(auditCtx, &auditModel.ListReq{
				ListParams: pageOf(100), EntityType: new(entityType),
			})
			require.NoError(t, err, name)
			assert.Empty(t, auditItems, "%s: записи %s не-админу не видны", name, entityType)
			assert.Zero(t, total, name)
		}

		// запись о создании самого пользователя — тоже
		auditItems, _, err = a.auditUsecase.List(auditCtx, &auditModel.ListReq{
			ListParams: pageOf(100), EntityId: new(fmt.Sprint(usrId)),
		})
		require.NoError(t, err, name)
		assert.Empty(t, auditItems, name)

		// свои приложения по фильтру видны
		auditItems, _, err = a.auditUsecase.List(auditCtx, &auditModel.ListReq{ListParams: pageOf(100), AppId: &appA.appId})
		require.NoError(t, err, name)
		assert.NotEmpty(t, auditItems, name)
	}

	// админ записи без приложения видит
	for _, entityType := range []string{auditModel.EntityUsr, auditModel.EntityApiKey} {
		adminAudit, _, err := a.auditUsecase.List(a.adminCtx(), &auditModel.ListReq{
			ListParams: pageOf(100), EntityType: new(entityType),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, adminAudit, entityType)
	}

	for _, runId := range []string{runA, runGlobal} {
		run, err := a.syncRunUsecase.Get(ctx, runId)
		require.NoError(t, err)
		assert.Len(t, run.Objects, 1)
	}

	_, _, err = a.kubeUsecase.Sync(ctx, nil)
	require.ErrorIs(t, err, errs.NotInCluster)

	// права администратора all_apps не даёт
	_, err = a.appUsecase.Create(ctx, &appModel.Edit{
		Active: new(true), Namespace: new("itest"), Name: new("x"), SlugName: new(a.name("allapps-new")),
	})
	require.ErrorIs(t, err, errs.NoPermission)
	_, _, err = a.usrUsecase.List(ctx, &usrModel.ListReq{})
	require.ErrorIs(t, err, errs.NoPermission)
	_, _, err = a.kubeUsecase.ListNamespaces(ctx)
	require.ErrorIs(t, err, errs.NoPermission)

	// флаг сняли — тем же токеном не видно уже ничего: права берутся из
	// записи на каждый запрос, а app_ids при Update заменяется (пусто)
	require.NoError(t, a.usrUsecase.Update(a.adminCtx(), usrId, &usrModel.Edit{AllApps: new(false)}))
	a.requireNothingVisible(t, a.tokenCtx(t, token), appA, appB)

	// флаг в аудите виден
	rows := a.auditRows(t, auditModel.EntityUsr, fmt.Sprint(usrId))
	require.NotEmpty(t, rows)
	assert.Contains(t, rows[len(rows)-1].changes, "all_apps")
}

// Поле all_apps проходит через создание, чтение и изменение пользователя;
// Update без all_apps флаг не трогает.
func TestUsrAllAppsField(t *testing.T) {
	a := newAccessStack(t)
	ctx := a.adminCtx()

	usrId, _ := a.newUser(t, "allapps-field", usrModel.Edit{AllApps: new(true), AppIds: []string{"some-app"}})

	item, err := a.usrUsecase.Get(ctx, usrId)
	require.NoError(t, err)
	assert.True(t, item.AllApps)
	assert.Equal(t, []string{"some-app"}, item.AppIds)

	// all_apps не передан — не меняется
	require.NoError(t, a.usrUsecase.Update(ctx, usrId, &usrModel.Edit{Name: new("renamed")}))
	item, err = a.usrUsecase.Get(ctx, usrId)
	require.NoError(t, err)
	assert.True(t, item.AllApps)

	require.NoError(t, a.usrUsecase.Update(ctx, usrId, &usrModel.Edit{AllApps: new(false)}))
	item, err = a.usrUsecase.Get(ctx, usrId)
	require.NoError(t, err)
	assert.False(t, item.AllApps)

	// по умолчанию — выключен
	defaultId, _ := a.newUser(t, "allapps-default", usrModel.Edit{})
	item, err = a.usrUsecase.Get(ctx, defaultId)
	require.NoError(t, err)
	assert.False(t, item.AllApps)
	assert.Empty(t, item.AppIds)
}

// Миграция 000012 сохраняет поведение существующих пользователей: не-админ
// с пустым app_ids (раньше — «доступ ко всем») получает all_apps=true,
// остальные — false. Выключенным пользователям миграция сразу сдвигает номер
// поколения сессий: их старые токены (считаются поколением 0) не должны
// заработать после повторного включения учётки. Заодно проверяется откат
// миграции.
func TestMigrationKeepsAccessOfExistingUsers(t *testing.T) {
	a := newAccessStack(t)
	ctx := context.Background()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationsPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "migrations")

	m, err := migrate.New("file://"+migrationsPath, os.Getenv("TEST_PG_DSN"))
	require.NoError(t, err)
	t.Cleanup(func() {
		// что бы ни случилось — база остаётся на последней версии
		if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
			t.Errorf("migrate up: %v", upErr)
		}
		_, _ = m.Close()
	})

	usrColumns := func(names ...string) int {
		t.Helper()
		var n int
		require.NoError(t, a.pool.QueryRow(ctx, `
			select count(*) from information_schema.columns
			where table_name = 'usr' and column_name = any($1)
		`, names).Scan(&n))
		return n
	}

	// 000013 убрала колонки 2FA, её откат возвращает их пустыми
	require.Zero(t, usrColumns("totp_secret", "totp_enabled"), "колонок 2FA нет")
	require.NoError(t, m.Migrate(12))
	require.Equal(t, 2, usrColumns("totp_secret", "totp_enabled"), "down 000013 возвращает колонки 2FA")

	// откат 000012: колонок all_apps и session_version нет
	require.NoError(t, m.Migrate(11))

	var columns int
	require.NoError(t, a.pool.QueryRow(ctx, `
		select count(*) from information_schema.columns
		where table_name = 'usr' and column_name in ('all_apps', 'session_version')
	`).Scan(&columns))
	require.Zero(t, columns, "down-миграция убирает обе колонки")

	legacyAll, legacyListed, legacyAdmin := a.name("legacy-all"), a.name("legacy-listed"), a.name("legacy-admin")
	legacyInactive, legacyInactiveAdmin := a.name("legacy-inactive"), a.name("legacy-inactive-admin")
	_, err = a.pool.Exec(ctx, `
		insert into usr (active, is_admin, name, username, password, app_ids) values
			(true,  false, $1, $1, 'x', '[]'::jsonb),
			(true,  false, $2, $2, 'x', '["some-app"]'::jsonb),
			(true,  true,  $3, $3, 'x', '[]'::jsonb),
			(false, false, $4, $4, 'x', '["some-app"]'::jsonb),
			(false, true,  $5, $5, 'x', '[]'::jsonb)
	`, legacyAll, legacyListed, legacyAdmin, legacyInactive, legacyInactiveAdmin)
	require.NoError(t, err)

	require.NoError(t, m.Steps(1))

	legacy := []string{legacyAll, legacyListed, legacyAdmin, legacyInactive, legacyInactiveAdmin}
	rows, err := a.pool.Query(ctx,
		`select username, all_apps, session_version from usr where username = any($1)`, legacy,
	)
	require.NoError(t, err)
	defer rows.Close()

	allApps := map[string]bool{}
	versions := map[string]int64{}
	for rows.Next() {
		var (
			username string
			flag     bool
			version  int64
		)
		require.NoError(t, rows.Scan(&username, &flag, &version))
		allApps[username] = flag
		versions[username] = version
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, map[string]bool{
		legacyAll:           true,  // пустой список у не-админа раньше значил «всё»
		legacyListed:        false, // доступ по списку остаётся по списку
		legacyAdmin:         false, // админу флаг не нужен
		legacyInactive:      false,
		legacyInactiveAdmin: false,
	}, allApps)

	assert.Equal(t, map[string]int64{
		// включённые: выданные до миграции токены (поколение 0) продолжают работать
		legacyAll:    0,
		legacyListed: 0,
		legacyAdmin:  0,
		// выключенные: поколение сдвинуто, старые токены не оживут
		legacyInactive:      1,
		legacyInactiveAdmin: 1,
	}, versions)

	// токены выключенного до миграции пользователя, выданные до неё (в них
	// нет номера поколения — считается 0), после включения учётки не работают
	var inactiveId int64
	var passwordHash string
	require.NoError(t, a.pool.QueryRow(ctx,
		`select id, password from usr where username = $1`, legacyInactive).Scan(&inactiveId, &passwordHash))

	legacyAccess, err := a.sessionSvc.CreateToken(inactiveId, 0)
	require.NoError(t, err)
	legacyRefresh, err := a.sessionSvc.CreateRefreshToken(inactiveId, passwordHash, 0)
	require.NoError(t, err)

	require.NoError(t, a.usrUsecase.Update(a.adminCtx(), inactiveId, &usrModel.Edit{Active: new(true), AppIds: []string{"some-app"}}))

	_, err = a.usrUsecase.SessionFromToken(ctx, legacyAccess)
	require.ErrorIs(t, err, errs.NotAuthorized, "старый access-токен после включения учётки")
	_, _, err = a.usrUsecase.RefreshToken(ctx, legacyRefresh)
	require.ErrorIs(t, err, errs.NotAuthorized, "старый refresh-токен после включения учётки")

	// у включённого до миграции пользователя токены поколения 0 — рабочие
	var activeId int64
	require.NoError(t, a.pool.QueryRow(ctx, `select id from usr where username = $1`, legacyListed).Scan(&activeId))
	activeAccess, err := a.sessionSvc.CreateToken(activeId, 0)
	require.NoError(t, err)
	_, err = a.usrUsecase.SessionFromToken(ctx, activeAccess)
	require.NoError(t, err)
}

// ── Сессии ──────────────────────────────────────────────

// Выключенный пользователь отвергается сразу, а не по истечении токена;
// включение обратно старые токены не оживляет.
func TestInactiveUserTokenRejected(t *testing.T) {
	a := newAccessStack(t)

	usrId, username := a.newUser(t, "inactive-user", usrModel.Edit{AllApps: new(true)})
	res := a.login(t, username, accessPassword)

	ctx := a.tokenCtx(t, res.Jwt)
	_, err := a.usrUsecase.GetProfile(ctx)
	require.NoError(t, err)

	require.NoError(t, a.usrUsecase.Update(a.adminCtx(), usrId, &usrModel.Edit{Active: new(false)}))

	_, err = a.usrUsecase.SessionFromToken(context.Background(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized, "access-токен выключенного пользователя")
	_, _, err = a.usrUsecase.RefreshToken(context.Background(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized, "refresh-токен выключенного пользователя")
	_, err = a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{Username: username, Password: accessPassword})
	require.ErrorIs(t, err, errs.NotAuthorized, "вход выключенного пользователя")

	require.NoError(t, a.usrUsecase.Update(a.adminCtx(), usrId, &usrModel.Edit{Active: new(true), AllApps: new(true)}))

	_, err = a.usrUsecase.SessionFromToken(context.Background(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized, "старый access-токен после повторного включения")
	_, _, err = a.usrUsecase.RefreshToken(context.Background(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized, "старый refresh-токен после повторного включения")

	// новый вход работает
	a.loginCtx(t, username)
}

// Токен со старым номером поколения сессий (после выхода или смены пароля)
// отвергается: и access, и refresh. session_version в базе растёт.
func TestStaleSessionVersionRejected(t *testing.T) {
	a := newAccessStack(t)

	usrId, username := a.newUser(t, "stale-user", usrModel.Edit{})

	version := func() int64 {
		t.Helper()
		var v int64
		require.NoError(t, a.pool.QueryRow(context.Background(),
			`select session_version from usr where id = $1`, usrId).Scan(&v))
		return v
	}
	requireRevoked := func(res *usrUsc.LoginResult, why string) {
		t.Helper()
		_, err := a.usrUsecase.SessionFromToken(context.Background(), res.Jwt)
		require.ErrorIs(t, err, errs.NotAuthorized, "access: "+why)
		_, _, err = a.usrUsecase.RefreshToken(context.Background(), res.RefreshToken)
		require.ErrorIs(t, err, errs.NotAuthorized, "refresh: "+why)
	}

	require.Zero(t, version())

	// выход гасит все сессии пользователя, в том числе на другом устройстве
	first := a.login(t, username, accessPassword)
	second := a.login(t, username, accessPassword)
	require.NoError(t, a.usrUsecase.Logout(a.tokenCtx(t, first.Jwt)))
	assert.EqualValues(t, 1, version())
	requireRevoked(first, "после выхода")
	requireRevoked(second, "после выхода на другом устройстве")

	// обновление токенов сохраняет поколение: новая пара рабочая
	third := a.login(t, username, accessPassword)
	access, refresh, err := a.usrUsecase.RefreshToken(context.Background(), third.RefreshToken)
	require.NoError(t, err)
	a.tokenCtx(t, access)
	assert.EqualValues(t, 1, version())

	// админ сменил пароль — сессии пользователя погашены
	const newPassword = "N3w-Access-Passw0rd!"
	require.NoError(t, a.usrUsecase.Update(a.adminCtx(), usrId, &usrModel.Edit{Password: new(newPassword)}))
	assert.EqualValues(t, 2, version())
	requireRevoked(third, "после смены пароля админом")
	requireRevoked(&usrUsc.LoginResult{Jwt: access, RefreshToken: refresh}, "обновлённая пара после смены пароля")

	// пользователь сменил пароль сам — погашена и текущая сессия
	fourth := a.login(t, username, newPassword)
	require.NoError(t, a.usrUsecase.UpdateProfile(a.tokenCtx(t, fourth.Jwt), &usrUsc.UpdateProfileReq{
		Password:        new(accessPassword),
		CurrentPassword: new(newPassword),
	}))
	assert.EqualValues(t, 3, version())
	requireRevoked(fourth, "после смены пароля самим пользователем")

	// правка без пароля сессии не трогает
	fifth := a.login(t, username, accessPassword)
	require.NoError(t, a.usrUsecase.Update(a.adminCtx(), usrId, &usrModel.Edit{Name: new("renamed")}))
	require.NoError(t, a.usrUsecase.UpdateProfile(a.tokenCtx(t, fifth.Jwt), &usrUsc.UpdateProfileReq{Name: new("renamed again")}))
	assert.EqualValues(t, 3, version())
	a.tokenCtx(t, fifth.Jwt)
}

// Смена пароля и логина в профиле — только с верным текущим паролем.
func TestProfileChangeRequiresCurrentPassword(t *testing.T) {
	a := newAccessStack(t)

	_, username := a.newUser(t, "profile-user", usrModel.Edit{})
	ctx := a.loginCtx(t, username)

	const newPassword = "N3w-Access-Passw0rd!"
	newUsername := a.name("profile-renamed")

	for name, req := range map[string]*usrUsc.UpdateProfileReq{
		"пароль без текущего":          {Password: new(newPassword)},
		"пароль с пустым текущим":      {Password: new(newPassword), CurrentPassword: new("")},
		"пароль с неверным текущим":    {Password: new(newPassword), CurrentPassword: new("Wrong-Passw0rd!")},
		"логин без текущего":           {Username: new(newUsername)},
		"логин с неверным текущим":     {Username: new(newUsername), CurrentPassword: new("Wrong-Passw0rd!")},
		"имя и логин без текущего":     {Name: new("x"), Username: new(newUsername)},
		"имя и пароль с неверным тек.": {Name: new("x"), Password: new(newPassword), CurrentPassword: new("Wrong-Passw0rd!")},
	} {
		require.ErrorIs(t, a.usrUsecase.UpdateProfile(ctx, req), errs.CurrentPasswordInvalid, name)
	}

	// ничего не изменилось: прежние логин и пароль работают, сессия жива
	profile, err := a.usrUsecase.GetProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, username, profile.Username)
	assert.Equal(t, username, profile.Name)
	a.login(t, username, accessPassword)

	// имя (и прежний логин, как его шлёт форма) — без текущего пароля
	require.NoError(t, a.usrUsecase.UpdateProfile(ctx, &usrUsc.UpdateProfileReq{
		Name: new("New Name"), Username: new(username),
	}))

	// логин и пароль — с верным текущим
	require.NoError(t, a.usrUsecase.UpdateProfile(ctx, &usrUsc.UpdateProfileReq{
		Username:        new(newUsername),
		Password:        new(newPassword),
		CurrentPassword: new(accessPassword),
	}))

	profile, err = a.usrUsecase.GetProfile(a.tokenCtx(t, a.login(t, newUsername, newPassword).Jwt))
	require.NoError(t, err)
	assert.Equal(t, newUsername, profile.Username)
	assert.Equal(t, "New Name", profile.Name)

	_, err = a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{Username: newUsername, Password: accessPassword})
	require.ErrorIs(t, err, errs.NotAuthorized, "старый пароль больше не подходит")
}

// Вход: пять неверных паролей подряд блокируют логин — и существующий, и
// несуществующий одинаково; верный пароль во время блокировки не принимается.
func TestLoginLockout(t *testing.T) {
	a := newAccessStack(t)

	_, username := a.newUser(t, "lockout-user", usrModel.Edit{})

	for _, login := range []string{username, a.name("lockout-ghost")} {
		for range 5 {
			_, err := a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{Username: login, Password: "Wrong-Passw0rd!"})
			require.ErrorIs(t, err, errs.NotAuthorized, login)
		}

		_, err := a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{Username: login, Password: accessPassword})
		fullErr, ok := errors.AsType[errs.ErrFull](err)
		require.True(t, ok, "expected errs.ErrFull, got: %v", err)
		assert.Equal(t, errs.TooManyAttempts, fullErr.Err, login)
		assert.Equal(t, "60", fullErr.Fields["retry_after"], login)
	}
}

// Логины в базе различаются регистром, а счётчик входа ведётся по
// пользователю: успешный вход под логином, отличающимся только регистром, —
// это вход другого пользователя, и счётчик неудач первого он не сбрасывает.
func TestLoginCounterNotResetByOtherCaseLogin(t *testing.T) {
	a := newAccessStack(t)

	_, victim := a.newUser(t, "case-admin", usrModel.Edit{IsAdmin: new(true)})

	// обычный пользователь называет себя так же, но в другом регистре —
	// уникальный индекс это допускает
	_, attacker := a.newUser(t, "case-attacker", usrModel.Edit{})
	twin := strings.ToUpper(victim)
	require.NotEqual(t, victim, twin)
	require.NoError(t, a.usrUsecase.UpdateProfile(a.loginCtx(t, attacker), &usrUsc.UpdateProfileReq{
		Username:        new(twin),
		CurrentPassword: new(accessPassword),
	}))

	wrong := func(username string) {
		t.Helper()
		_, err := a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{Username: username, Password: "Wrong-Passw0rd!"})
		require.ErrorIs(t, err, errs.NotAuthorized, username)
	}

	for range 4 {
		wrong(victim)
	}

	// вход «двойника» — успех другого пользователя
	a.login(t, twin, accessPassword)

	// пятая неудача по жертве — блокировка
	wrong(victim)
	_, err := a.usrUsecase.Login(context.Background(), &usrUsc.LoginReq{Username: victim, Password: accessPassword})
	fullErr, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "expected errs.ErrFull, got: %v", err)
	assert.Equal(t, errs.TooManyAttempts, fullErr.Err)

	// «двойник» при этом входит свободно
	a.login(t, twin, accessPassword)
}

// Свои пароль и логин админ через управление пользователями не меняет:
// только в профиле, где нужен текущий пароль.
func TestAdminCannotChangeOwnCredentialsViaUsrUpdate(t *testing.T) {
	a := newAccessStack(t)

	adminId, adminUsername := a.newUser(t, "self-admin", usrModel.Edit{IsAdmin: new(true)})
	otherId, otherUsername := a.newUser(t, "self-other", usrModel.Edit{})
	ctx := a.loginCtx(t, adminUsername)

	requireSelfDenied := func(err error, what string) {
		t.Helper()
		fullErr, ok := errors.AsType[errs.ErrFull](err)
		require.True(t, ok, "%s: expected errs.ErrFull, got: %v", what, err)
		assert.Equal(t, errs.NoPermission, fullErr.Err, what)
		assert.Equal(t, "change your own password and username in the profile", fullErr.Desc, what)
	}

	const newPassword = "N3w-Access-Passw0rd!"

	requireSelfDenied(a.usrUsecase.Update(ctx, adminId, &usrModel.Edit{Password: new(newPassword)}), "свой пароль")
	requireSelfDenied(a.usrUsecase.Update(ctx, adminId, &usrModel.Edit{Username: new(a.name("self-renamed"))}), "свой логин")

	// прежние логин и пароль работают, сессия жива
	a.login(t, adminUsername, accessPassword)
	_, err := a.usrUsecase.GetProfile(ctx)
	require.NoError(t, err)

	// остальные свои поля и прежний логин — можно
	require.NoError(t, a.usrUsecase.Update(ctx, adminId, &usrModel.Edit{
		Name: new("Self Admin"), Username: new(adminUsername), IsAdmin: new(true),
	}))
	self, err := a.usrUsecase.Get(ctx, adminId)
	require.NoError(t, err)
	assert.Equal(t, "Self Admin", self.Name)
	assert.Equal(t, adminUsername, self.Username)

	// чужую учётку — как раньше
	require.NoError(t, a.usrUsecase.Update(ctx, otherId, &usrModel.Edit{Password: new(newPassword)}))
	a.login(t, otherUsername, newPassword)

	// в профиле с текущим паролем свой пароль меняется
	require.NoError(t, a.usrUsecase.UpdateProfile(ctx, &usrUsc.UpdateProfileReq{
		Password: new(newPassword), CurrentPassword: new(accessPassword),
	}))
	a.login(t, adminUsername, newPassword)
}

// ── Сессии по ключу и права ─────────────────────────────

// Сессия по API-ключу — даже ключу админа — не меняет профиль, не выпускает
// ключи и не управляет пользователями. Читать может.
func TestApiKeySessionCannotManageAccounts(t *testing.T) {
	a := newAccessStack(t)

	adminId, adminUsername := a.newUser(t, "key-admin", usrModel.Edit{IsAdmin: new(true)})
	victimId, _ := a.newUser(t, "key-victim", usrModel.Edit{})
	humanCtx := a.loginCtx(t, adminUsername)

	keyId, key, err := a.apikeyUsecase.Create(humanCtx, "ci", nil, constant.ApiKeyScopeFull, accessPassword)
	require.NoError(t, err)

	keySession, err := a.apikeyUsecase.SessionFromKey(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, adminId, keySession.Id)
	require.True(t, keySession.Admin)
	require.Equal(t, keyId, keySession.ApiKeyId)
	keyCtx := a.sessionSvc.WithContext(context.Background(), keySession)

	calls := map[string]func() error{
		"UpdateProfile (пароль)": func() error {
			return a.usrUsecase.UpdateProfile(keyCtx, &usrUsc.UpdateProfileReq{
				Password: new("N3w-Access-Passw0rd!"), CurrentPassword: new(accessPassword),
			})
		},
		"UpdateProfile (имя)": func() error {
			return a.usrUsecase.UpdateProfile(keyCtx, &usrUsc.UpdateProfileReq{Name: new("x")})
		},
		"Logout": func() error { return a.usrUsecase.Logout(keyCtx) },
		"Usr.Create": func() error {
			_, err := a.usrUsecase.Create(keyCtx, &usrModel.Edit{
				Name: new("eve"), Username: new(a.name("key-eve")), Password: new(accessPassword), IsAdmin: new(true),
			})
			return err
		},
		"Usr.Update": func() error { return a.usrUsecase.Update(keyCtx, victimId, &usrModel.Edit{IsAdmin: new(true)}) },
		"Usr.Delete": func() error { return a.usrUsecase.Delete(keyCtx, victimId) },
		"ApiKey.Create": func() error {
			_, _, err := a.apikeyUsecase.Create(keyCtx, "more", nil, constant.ApiKeyScopeFull, accessPassword)
			return err
		},
		"ApiKey.Create (mcp_only)": func() error {
			_, _, err := a.apikeyUsecase.Create(keyCtx, "more", nil, constant.ApiKeyScopeMcpOnly, accessPassword)
			return err
		},
		"ApiKey.Update": func() error { return a.apikeyUsecase.Update(keyCtx, keyId, nil, new("renamed"), nil) },
		"ApiKey.Delete": func() error { return a.apikeyUsecase.Delete(keyCtx, keyId) },
	}
	for name, call := range calls {
		require.ErrorIs(t, call(), errs.NoPermission, name)
	}

	// ничего не изменилось
	victim, err := a.usrUsecase.Get(humanCtx, victimId)
	require.NoError(t, err)
	assert.False(t, victim.IsAdmin)
	a.login(t, adminUsername, accessPassword)

	keys, _, err := a.apikeyUsecase.List(keyCtx, &apikeyModel.ListReq{ListParams: pageOf(100), UsrId: &adminId})
	require.NoError(t, err)
	require.Len(t, keys, 1, "чтение ключей оставлено, новых ключей нет")
	assert.Equal(t, "ci", keys[0].Name)

	// чтение оставлено
	profile, err := a.usrUsecase.GetProfile(keyCtx)
	require.NoError(t, err)
	assert.Equal(t, adminUsername, profile.Username)

	// из входа в админку тот же админ всё это делает
	require.NoError(t, a.apikeyUsecase.Update(humanCtx, keyId, nil, new("renamed"), nil))
	require.NoError(t, a.apikeyUsecase.Delete(humanCtx, keyId))

	// удалённый ключ больше не принимается
	_, err = a.apikeyUsecase.SessionFromKey(context.Background(), key)
	require.ErrorIs(t, err, errs.NotAuthorized)
}

// Список пользователей и чужие учётки не-админу недоступны.
func TestUsrListAdminOnly(t *testing.T) {
	a := newAccessStack(t)

	otherId, _ := a.newUser(t, "list-other", usrModel.Edit{})
	usrId, username := a.newUser(t, "list-user", usrModel.Edit{AllApps: new(true)})
	_, adminUsername := a.newUser(t, "list-admin", usrModel.Edit{IsAdmin: new(true)})

	ctx := a.loginCtx(t, username)

	_, _, err := a.usrUsecase.List(ctx, &usrModel.ListReq{})
	require.ErrorIs(t, err, errs.NoPermission)
	_, _, err = a.usrUsecase.List(ctx, &usrModel.ListReq{Search: &a.suffix})
	require.ErrorIs(t, err, errs.NoPermission)
	_, err = a.usrUsecase.Get(ctx, otherId)
	require.ErrorIs(t, err, errs.NoPermission)
	_, err = a.usrUsecase.Get(ctx, usrId)
	require.ErrorIs(t, err, errs.NoPermission, "себя — через GetProfile")

	profile, err := a.usrUsecase.GetProfile(ctx)
	require.NoError(t, err)
	assert.Equal(t, username, profile.Username)

	// без входа
	_, _, err = a.usrUsecase.List(context.Background(), &usrModel.ListReq{})
	require.ErrorIs(t, err, errs.NotAuthorized)

	// админ видит
	adminCtx := a.loginCtx(t, adminUsername)
	items, _, err := a.usrUsecase.List(adminCtx, &usrModel.ListReq{Search: &a.suffix})
	require.NoError(t, err)
	assert.Len(t, items, 3)
	for _, item := range items {
		assert.Empty(t, item.Password)
	}
	other, err := a.usrUsecase.Get(adminCtx, otherId)
	require.NoError(t, err)
	assert.Empty(t, other.Password)
}
