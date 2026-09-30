package kube

import (
	"context"
	"errors"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	"github.com/rendau/kusec/internal/errs"
)

// importSkippedConfigMaps — configmap-ы, не предназначенные для импорта:
// kube-root-ca.crt k8s сам кладёт в каждый namespace.
var importSkippedConfigMaps = map[string]bool{
	"kube-root-ca.crt": true,
}

// ListClusterConfigMaps возвращает configmap-ы кластера для выбора при
// импорте: без системных namespace-ов (kube-*) и без служебных
// (kube-root-ca.crt). Значения не отдаются — только ключи data и binaryData.
// Вне кластера это штатная ситуация: inCluster=false и пустой список без
// ошибки.
func (s *Service) ListClusterConfigMaps(ctx context.Context, namespace string) ([]*ClusterConfigMap, bool, error) {
	client, err := s.getClient()
	if err != nil {
		if errors.Is(err, errs.NotInCluster) {
			return nil, false, nil
		}
		return nil, false, err
	}

	ns := metav1.NamespaceAll
	if namespace != "" {
		ns = namespace
	}

	list, err := client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("k8s: list configmaps: %w", err)
	}

	result := make([]*ClusterConfigMap, 0, len(list.Items))
	for i := range list.Items {
		cm := &list.Items[i]
		if systemNamespaces[cm.Namespace] || importSkippedConfigMaps[cm.Name] {
			continue
		}

		result = append(result, &ClusterConfigMap{
			Namespace: cm.Namespace,
			Name:      cm.Name,
			Keys:      sortedConfigMapKeys(cm.Data, cm.BinaryData),
			Managed:   cm.Labels[managedByLabelKey] == managedByLabelValue,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		return result[i].Name < result[j].Name
	})

	return result, true, nil
}

// ImportConfigMap переносит один configmap кластера в указанное приложение:
// он становится записью configmap в appId с item-ами по ключам data (текст,
// encoding=plain) и binaryData (encoding=base64). Семантика slug, дозаполнения
// и opts — как у ImportSecret. Источник в кластере не изменяется.
func (s *Service) ImportConfigMap(ctx context.Context, appId string, ref ImportRef, configMapSlug string, opts ImportOptions) (*ImportResult, error) {
	return s.importSingle(ctx, appId, ImportSpec{Ref: ref, Slug: configMapSlug, Options: opts}, s.importConfigMapOne)
}

// ImportConfigMaps — пакетный импорт: несколько configmap-ов кластера в одно
// приложение за один вызов (см. importBatch).
func (s *Service) ImportConfigMaps(ctx context.Context, appId string, specs []ImportSpec) ([]ImportBatchItem, error) {
	return s.importBatch(ctx, appId, specs, s.importConfigMapOne)
}

// importConfigMapOne — importOneFn для configmap-а: находит или создаёт
// посадочный configmap и переносит ключи data/binaryData.
func (s *Service) importConfigMapOne(ctx context.Context, client kubernetes.Interface, app *appModel.Main, spec ImportSpec, batchId *string) (*ImportResult, error) {
	kcm, err := client.CoreV1().ConfigMaps(spec.Ref.Namespace).Get(ctx, spec.Ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get cluster configmap %s/%s: %w", spec.Ref.Namespace, spec.Ref.Name, err)
	}

	slug, err := importSlug(spec.Slug, "configmap")
	if err != nil {
		return nil, err
	}

	// План проверяется до любых мутаций.
	plan, err := importKeyPlan(sortedConfigMapKeys(kcm.Data, kcm.BinaryData), spec.Options.Keys)
	if err != nil {
		return nil, err
	}

	existing, err := s.findConfigMapBySlug(ctx, app.Id, slug)
	if err != nil {
		return nil, err
	}

	result := newImportResult(slug)

	// Мутации и записи аудита — в одной транзакции.
	err = s.txm.TxFn(ctx, func(ctx context.Context) error {
		// existingItems: ключ → item (active и неактивные — чтобы найти любой
		// совпавший ключ и не плодить дубли).
		existingItems := map[string]*configitemModel.Main{}
		if existing != nil {
			result.ObjectId = existing.Id
			result.KubeName = ConfigMapName(app.SlugName, existing.SlugName, existing.ExactSlug)

			items, _, err := s.configItemSvc.List(ctx, &configitemModel.ListReq{ConfigMapId: new(existing.Id)})
			if err != nil {
				return fmt.Errorf("list existing config items: %w", err)
			}
			for _, it := range items {
				existingItems[it.Key] = it
			}
		} else {
			configMapId, err := s.configMapSvc.Create(ctx, &configmapModel.Edit{
				AppId:       new(app.Id),
				Active:      new(true),
				SlugName:    new(slug),
				Description: new(fmt.Sprintf("Imported from %s/%s", spec.Ref.Namespace, spec.Ref.Name)),
			})
			if err != nil {
				return fmt.Errorf("create configmap: %w", err)
			}
			result.ObjectId = configMapId
			result.ObjectCreated = true
			result.KubeName = ConfigMapName(app.SlugName, slug, false)

			created, _, err := s.configMapSvc.Get(ctx, configMapId, true)
			if err != nil {
				return fmt.Errorf("get created configmap: %w", err)
			}
			if err = s.auditRec.RecordConfigMap(ctx, nil, created, importAction, batchId); err != nil {
				return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
			}
		}

		return applyImportPlan(result, plan, spec.Options.Overwrite,
			func(itemKey string) (string, bool) {
				item, ok := existingItems[itemKey]
				if !ok {
					return "", false
				}
				return item.Value, true
			},
			func(entry importKeyEntry) error {
				value, encoding := configMapImportValue(kcm, entry.sourceKey)
				return s.importCreateConfigItem(ctx, result.ObjectId, entry.itemKey, value, encoding, batchId)
			},
			func(entry importKeyEntry) error {
				value, encoding := configMapImportValue(kcm, entry.sourceKey)
				return s.importUpdateConfigItem(ctx, existingItems[entry.itemKey], value, encoding, batchId)
			},
		)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// configMapImportValue — значение ключа configmap-а в виде для хранения:
// data — текст как есть (plain), binaryData — через encodeImportValue.
// Зеркально buildConfigMapData и GetClusterConfigMap.
func configMapImportValue(kcm *corev1.ConfigMap, key string) (value string, encoding string) {
	if text, ok := kcm.Data[key]; ok {
		return text, "plain"
	}
	return encodeImportValue(kcm.BinaryData[key])
}

// importCreateConfigItem создаёт config item по ключу источника и пишет
// запись аудита.
func (s *Service) importCreateConfigItem(ctx context.Context, configMapId, key, value, encoding string, batchId *string) error {
	itemId, err := s.configItemSvc.Create(ctx, &configitemModel.Edit{
		ConfigMapId: new(configMapId),
		Active:      new(true),
		Key:         new(key),
		Value:       new(value),
		Encoding:    new(encoding),
	})
	if err != nil {
		return fmt.Errorf("key %q: create config item: %w", key, err)
	}
	created, _, err := s.configItemSvc.Get(ctx, itemId, true)
	if err != nil {
		return fmt.Errorf("key %q: get created config item: %w", key, err)
	}
	if err = s.auditRec.RecordConfigItem(ctx, nil, created, importAction, batchId); err != nil {
		return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
	}
	return nil
}

// importUpdateConfigItem перезаписывает значение существующего config item-а
// значением из кластера и пишет запись аудита.
func (s *Service) importUpdateConfigItem(ctx context.Context, old *configitemModel.Main, value, encoding string, batchId *string) error {
	err := s.configItemSvc.Update(ctx, old.Id, &configitemModel.Edit{
		Value:    new(value),
		Encoding: new(encoding),
	})
	if err != nil {
		return fmt.Errorf("key %q: update config item: %w", old.Key, err)
	}
	updated, _, err := s.configItemSvc.Get(ctx, old.Id, true)
	if err != nil {
		return fmt.Errorf("key %q: get updated config item: %w", old.Key, err)
	}
	if err = s.auditRec.RecordConfigItem(ctx, old, updated, importAction, batchId); err != nil {
		return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
	}
	return nil
}

func (s *Service) findConfigMapBySlug(ctx context.Context, appId, slug string) (*configmapModel.Main, error) {
	configMaps, _, err := s.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, fmt.Errorf("configMapSvc.List: %w", err)
	}
	for _, configMap := range configMaps {
		if configMap.SlugName == slug {
			return configMap, nil
		}
	}
	return nil, nil
}
