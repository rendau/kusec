package monitoring

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rendau/kusec/internal/config"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
	kubeService "github.com/rendau/kusec/internal/service/kube"
	"github.com/rendau/kusec/internal/util"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
)

// Usecase — read-only ручки мониторинга: resolve по имени k8s-объекта,
// ключи app без значений, сравнение kusec ↔ кластер.
type Usecase struct {
	appSvc        AppServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI
	auditSvc      AuditServiceI
	kubeSvc       KubeServiceI
	sessionSvc    SessionServiceI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
	auditSvc AuditServiceI,
	kubeSvc KubeServiceI,
	sessionSvc SessionServiceI,
) *Usecase {
	return &Usecase{
		appSvc:        appSvc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
		auditSvc:      auditSvc,
		kubeSvc:       kubeSvc,
		sessionSvc:    sessionSvc,
	}
}

// Resolve находит app и его secret/configmap по имени k8s-объекта в
// namespace (например kusec-caravan-main из envFrom пода или аннотации
// reloader). Не найдено — (nil, nil): это не ошибка.
func (u *Usecase) Resolve(ctx context.Context, namespace, kubeName string) (*ResolveResult, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if namespace == "" || kubeName == "" {
		return nil, errs.InvalidRequest
	}

	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{Namespace: &namespace})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}

	session := u.sessionSvc.FromContext(ctx)

	for _, app := range apps {
		if !session.HasAppAccess(app.Id) {
			continue
		}

		secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(app.Id)})
		if err != nil {
			return nil, fmt.Errorf("secretSvc.List: %w", err)
		}
		for _, secret := range secrets {
			if kubeService.SecretName(app.SlugName, secret.SlugName, secret.ExactSlug) == kubeName {
				return &ResolveResult{
					App:        app,
					KubeKind:   "Secret",
					ObjectId:   secret.Id,
					ObjectSlug: secret.SlugName,
				}, nil
			}
		}

		configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: new(app.Id)})
		if err != nil {
			return nil, fmt.Errorf("configMapSvc.List: %w", err)
		}
		for _, configMap := range configMaps {
			if kubeService.ConfigMapName(app.SlugName, configMap.SlugName, configMap.ExactSlug) == kubeName {
				return &ResolveResult{
					App:        app,
					KubeKind:   "ConfigMap",
					ObjectId:   configMap.Id,
					ObjectSlug: configMap.SlugName,
				}, nil
			}
		}
	}

	return nil, nil
}

// requireApp — app с проверкой доступа сессии.
func (u *Usecase) requireApp(ctx context.Context, appId string) (*appModel.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if appId == "" {
		return nil, errs.IdRequired
	}
	if !u.sessionSvc.FromContext(ctx).HasAppAccess(appId) {
		return nil, errs.NoPermission
	}

	app, _, err := u.appSvc.Get(ctx, appId, true)
	if err != nil {
		return nil, fmt.Errorf("appSvc.Get: %w", err)
	}
	return app, nil
}

// appObjects — все secret/configmap приложения с их ключами.
func (u *Usecase) appObjects(ctx context.Context, appId string) (
	[]*secretModel.Main, map[string][]*itemModel.Main,
	[]*configmapModel.Main, map[string][]*configitemModel.Main, error,
) {
	secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("secretSvc.List: %w", err)
	}
	items, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("itemSvc.List: %w", err)
	}
	itemsBySecret := map[string][]*itemModel.Main{}
	for _, item := range items {
		itemsBySecret[item.SecretId] = append(itemsBySecret[item.SecretId], item)
	}

	configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("configMapSvc.List: %w", err)
	}
	configItems, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("configItemSvc.List: %w", err)
	}
	configItemsByMap := map[string][]*configitemModel.Main{}
	for _, configItem := range configItems {
		configItemsByMap[configItem.ConfigMapId] = append(configItemsByMap[configItem.ConfigMapId], configItem)
	}

	return secrets, itemsBySecret, configMaps, configItemsByMap, nil
}

// Keys возвращает все ключи app (secret + configmap) без значений.
func (u *Usecase) Keys(ctx context.Context, appId string) ([]*AppKey, error) {
	app, err := u.requireApp(ctx, appId)
	if err != nil {
		return nil, err
	}

	secrets, itemsBySecret, configMaps, configItemsByMap, err := u.appObjects(ctx, appId)
	if err != nil {
		return nil, err
	}

	editors, err := u.auditSvc.LastEditors(ctx, appId)
	if err != nil {
		return nil, fmt.Errorf("auditSvc.LastEditors: %w", err)
	}

	keys := make([]*AppKey, 0)

	for _, secret := range secrets {
		kubeName := kubeService.SecretName(app.SlugName, secret.SlugName, secret.ExactSlug)
		for _, item := range itemsBySecret[secret.Id] {
			keys = append(keys, &AppKey{
				Key:       item.Key,
				KubeKind:  "Secret",
				KubeName:  kubeName,
				IsSecret:  true,
				Active:    item.Active && secret.Active,
				ValueSize: int64(len(item.Value)),
				ValueHash: util.ValueFingerprint(config.Conf.AuditHashKey, item.Value),
				UpdatedAt: item.UpdatedAt,
				UpdatedBy: editors[item.Id],
				Synced:    syncedAt(secret.LastSyncedAt, item.UpdatedAt),
				ItemId:    item.Id,
				ParentId:  secret.Id,
			})
		}
	}

	for _, configMap := range configMaps {
		kubeName := kubeService.ConfigMapName(app.SlugName, configMap.SlugName, configMap.ExactSlug)
		for _, configItem := range configItemsByMap[configMap.Id] {
			keys = append(keys, &AppKey{
				Key:       configItem.Key,
				KubeKind:  "ConfigMap",
				KubeName:  kubeName,
				IsSecret:  false,
				Active:    configItem.Active && configMap.Active,
				ValueSize: int64(len(configItem.Value)),
				ValueHash: util.ValueFingerprint(config.Conf.AuditHashKey, configItem.Value),
				UpdatedAt: configItem.UpdatedAt,
				UpdatedBy: editors[configItem.Id],
				Synced:    syncedAt(configMap.LastSyncedAt, configItem.UpdatedAt),
				ItemId:    configItem.Id,
				ParentId:  configMap.Id,
			})
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].KubeName != keys[j].KubeName {
			return keys[i].KubeName < keys[j].KubeName
		}
		return keys[i].Key < keys[j].Key
	})

	return keys, nil
}

// syncedAt — изменение применено: последний sync объекта был не раньше него.
func syncedAt(lastSyncedAt *time.Time, updatedAt time.Time) bool {
	return lastSyncedAt != nil && !updatedAt.After(*lastSyncedAt)
}

// Drift сравнивает kusec ↔ кластер по всем secret/configmap приложения.
// inCluster=false — kusec запущен вне кластера, сравнение недоступно.
func (u *Usecase) Drift(ctx context.Context, appId string) ([]*DriftObject, bool, error) {
	app, err := u.requireApp(ctx, appId)
	if err != nil {
		return nil, false, err
	}

	secrets, itemsBySecret, configMaps, configItemsByMap, err := u.appObjects(ctx, appId)
	if err != nil {
		return nil, false, err
	}

	objects := make([]*DriftObject, 0, len(secrets)+len(configMaps))

	for _, secret := range secrets {
		kubeName := kubeService.SecretName(app.SlugName, secret.SlugName, secret.ExactSlug)

		cluster, inCluster, found, err := u.kubeSvc.GetClusterSecret(ctx, app.Namespace, kubeName)
		if err != nil {
			return nil, false, fmt.Errorf("kubeSvc.GetClusterSecret: %w", err)
		}
		if !inCluster {
			return nil, false, nil
		}

		objects = append(objects, driftObject(
			"Secret", kubeName, app.Namespace, secret.Id, secret.Active, secret.LastSyncedAt,
			desiredSecretValues(secret, itemsBySecret[secret.Id]),
			latestChange(secret.UpdatedAt, itemUpdatedAts(itemsBySecret[secret.Id])),
			cluster, found,
		))
	}

	for _, configMap := range configMaps {
		kubeName := kubeService.ConfigMapName(app.SlugName, configMap.SlugName, configMap.ExactSlug)

		cluster, inCluster, found, err := u.kubeSvc.GetClusterConfigMap(ctx, app.Namespace, kubeName)
		if err != nil {
			return nil, false, fmt.Errorf("kubeSvc.GetClusterConfigMap: %w", err)
		}
		if !inCluster {
			return nil, false, nil
		}

		objects = append(objects, driftObject(
			"ConfigMap", kubeName, app.Namespace, configMap.Id, configMap.Active, configMap.LastSyncedAt,
			desiredConfigValues(configMap, configItemsByMap[configMap.Id]),
			latestChange(configMap.UpdatedAt, configItemUpdatedAts(configItemsByMap[configMap.Id])),
			cluster, found,
		))
	}

	return objects, true, nil
}

// desiredSecretValues — желаемое содержимое k8s-секрета: active-ключи
// active-секрета (иначе объект не должен существовать — пустая карта).
func desiredSecretValues(secret *secretModel.Main, items []*itemModel.Main) map[string][]byte {
	desired := map[string][]byte{}
	if !secret.Active {
		return desired
	}
	for _, item := range items {
		if !item.Active {
			continue
		}
		desired[item.Key] = decodeStoredValue(item.Value, item.Encoding)
	}
	return desired
}

func desiredConfigValues(configMap *configmapModel.Main, items []*configitemModel.Main) map[string][]byte {
	desired := map[string][]byte{}
	if !configMap.Active {
		return desired
	}
	for _, item := range items {
		if !item.Active {
			continue
		}
		desired[item.Key] = decodeStoredValue(item.Value, item.Encoding)
	}
	return desired
}

// decodeStoredValue — байты значения записи kusec: encoding=base64 хранит
// base64 (файлы), иначе текст как есть. Невалидный base64 сравнивается как
// сырая строка (при sync такой ключ и не попал бы в кластер).
func decodeStoredValue(value, encoding string) []byte {
	if encoding == "base64" {
		if raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value)); err == nil {
			return raw
		}
	}
	return []byte(value)
}

// clusterValues — байты значений живого k8s-объекта (зеркально
// encodeImportValue: encoding=base64 — бинарные данные).
func clusterValues(cluster *kubeService.ClusterResource) map[string][]byte {
	values := map[string][]byte{}
	if cluster == nil {
		return values
	}
	for _, item := range cluster.Items {
		values[item.Key] = decodeStoredValue(item.Value, item.Encoding)
	}
	return values
}

func driftObject(
	kubeKind, kubeName, namespace, objectId string,
	active bool,
	lastSyncedAt *time.Time,
	desired map[string][]byte,
	lastChangedAt time.Time,
	cluster *kubeService.ClusterResource,
	found bool,
) *DriftObject {
	obj := &DriftObject{
		KubeKind:         kubeKind,
		KubeName:         kubeName,
		Namespace:        namespace,
		ObjectId:         objectId,
		ExistsInCluster:  found,
		MissingInCluster: []string{},
		ExtraInCluster:   []string{},
		ValueDiffers:     []string{},
	}
	if found {
		obj.Managed = cluster.Managed
	}

	live := clusterValues(cluster)
	if !found {
		live = map[string][]byte{}
	}

	for key, wantValue := range desired {
		liveValue, ok := live[key]
		switch {
		case !ok:
			obj.MissingInCluster = append(obj.MissingInCluster, key)
		case !bytes.Equal(liveValue, wantValue):
			obj.ValueDiffers = append(obj.ValueDiffers, key)
		}
	}
	for key := range live {
		if _, ok := desired[key]; !ok {
			obj.ExtraInCluster = append(obj.ExtraInCluster, key)
		}
	}
	sort.Strings(obj.MissingInCluster)
	sort.Strings(obj.ExtraInCluster)
	sort.Strings(obj.ValueDiffers)

	// изменено в kusec после последнего sync (или ни разу не синкалось,
	// а объект должен существовать)
	if lastSyncedAt == nil {
		if active {
			obj.NotSyncedSince = new(lastChangedAt)
		}
	} else if lastChangedAt.After(*lastSyncedAt) {
		obj.NotSyncedSince = new(lastChangedAt)
	}

	return obj
}

// latestChange — максимум из времени изменения объекта и его ключей.
func latestChange(objectUpdatedAt time.Time, itemTimes []time.Time) time.Time {
	latest := objectUpdatedAt
	for _, t := range itemTimes {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

func itemUpdatedAts(items []*itemModel.Main) []time.Time {
	times := make([]time.Time, 0, len(items))
	for _, item := range items {
		times = append(times, item.UpdatedAt)
	}
	return times
}

func configItemUpdatedAts(items []*configitemModel.Main) []time.Time {
	times := make([]time.Time, 0, len(items))
	for _, item := range items {
		times = append(times, item.UpdatedAt)
	}
	return times
}
