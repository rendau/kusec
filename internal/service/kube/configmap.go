package kube

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	configitemModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	"github.com/mechta-market/kusec/internal/errs"
)

// SyncConfigMaps приводит k8s-configmap-ы в соответствие с базой:
// создаёт новые, обновляет изменившиеся, удаляет лишние. Управляются только
// configmap-ы с лейблом app.kubernetes.io/managed-by=kusec — чужие объекты
// кластера не трогаются. Ошибки отдельных configmap-ов собираются в результат,
// не прерывая синхронизацию остальных.
func (s *Service) SyncConfigMaps(ctx context.Context, appIds []string) (*SyncResult, error) {
	if !s.mu.TryLock() {
		return nil, errs.SyncInProgress
	}
	defer s.mu.Unlock()

	client, err := s.getClient()
	if err != nil {
		return nil, err
	}

	result := &SyncResult{}

	desired, err := s.buildDesiredConfigMaps(ctx, result, appIds)
	if err != nil {
		return nil, err
	}

	existingList, err := client.CoreV1().ConfigMaps(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: managedBySelector,
	})
	if err != nil {
		return nil, fmt.Errorf("k8s: list managed configmaps: %w", err)
	}

	scopeSet := lo.SliceToMap(appIds, func(appId string) (string, bool) { return appId, true })

	existing := make(map[string]*corev1.ConfigMap, len(existingList.Items))
	for i := range existingList.Items {
		configMap := &existingList.Items[i]
		if !scopeSet[configMap.Annotations[appIdAnnotation]] {
			continue
		}
		existing[configMap.Namespace+"/"+configMap.Name] = configMap
	}

	ensuredNamespaces := map[string]bool{}

	for key, want := range desired {
		current, found := existing[key]
		delete(existing, key) // оставшиеся в existing будут удалены

		if !found {
			if err = s.ensureNamespace(ctx, client, want.namespace, ensuredNamespaces); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", key, err))
				continue
			}
			if _, err = client.CoreV1().ConfigMaps(want.namespace).Create(ctx, buildConfigMap(want), metav1.CreateOptions{}); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: create: %v", key, err))
				continue
			}
			result.Created = append(result.Created, key)
			continue
		}

		if configMapUpToDate(current, want) {
			result.Unchanged++
			continue
		}

		updated := current.DeepCopy()
		updated.Data = want.data
		updated.BinaryData = want.binaryData
		if updated.Annotations == nil {
			updated.Annotations = map[string]string{}
		}
		updated.Annotations[appIdAnnotation] = want.appId
		updated.Annotations[configMapIdAnnotation] = want.configMapId

		if _, err = client.CoreV1().ConfigMaps(want.namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: update: %v", key, err))
			continue
		}
		result.Updated = append(result.Updated, key)
	}

	// Управляемые configmap-ы, которым больше нет активных записей в базе.
	for key, stale := range existing {
		err = client.CoreV1().ConfigMaps(stale.Namespace).Delete(ctx, stale.Name, metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: delete: %v", key, err))
			continue
		}
		result.Deleted = append(result.Deleted, key)
	}

	sort.Strings(result.Created)
	sort.Strings(result.Updated)
	sort.Strings(result.Deleted)
	sort.Strings(result.Errors)

	return result, nil
}

// buildDesiredConfigMaps собирает желаемое состояние из базы: только
// active-записи (app, configmap, config_item). Невалидные configmap-ы
// пропускаются с ошибкой в result.
func (s *Service) buildDesiredConfigMaps(ctx context.Context, result *SyncResult, appIds []string) (map[string]*desiredConfigMap, error) {
	apps, _, err := s.appSvc.List(ctx, &appModel.ListReq{Ids: appIds, Active: new(true)})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}

	desired := make(map[string]*desiredConfigMap)

	for _, app := range apps {
		if errMsgs := validation.IsDNS1123Label(app.Namespace); len(errMsgs) > 0 {
			result.Errors = append(result.Errors,
				fmt.Sprintf("app %q: invalid namespace %q: %s", app.SlugName, app.Namespace, strings.Join(errMsgs, "; ")))
			continue
		}

		configMaps, _, err := s.configMapSvc.List(ctx, &configmapModel.ListReq{
			AppId:  new(app.Id),
			Active: new(true),
		})
		if err != nil {
			return nil, fmt.Errorf("configMapSvc.List: %w", err)
		}

		for _, configMap := range configMaps {
			name := ConfigMapName(app.SlugName, configMap.SlugName)
			key := app.Namespace + "/" + name

			if errMsgs := validation.IsDNS1123Subdomain(name); len(errMsgs) > 0 {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: invalid configmap name: %s", key, strings.Join(errMsgs, "; ")))
				continue
			}
			if clash, ok := desired[key]; ok {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: name collision between app ids %s and %s", key, clash.appId, app.Id))
				continue
			}

			data, binaryData, err := s.buildConfigMapData(ctx, configMap.Id)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", key, err))
				continue
			}

			desired[key] = &desiredConfigMap{
				namespace:   app.Namespace,
				name:        name,
				appId:       app.Id,
				configMapId: configMap.Id,
				data:        data,
				binaryData:  binaryData,
			}
		}
	}

	return desired, nil
}

// buildConfigMapData собирает data/binaryData configmap-а из active-items.
// encoding=base64 — значение хранится в base64 (файлы) и декодируется в байты
// (BinaryData); иначе значение кладётся как текст (Data). value_format
// (text/yaml/json) — подсказка редактора и на содержимое не влияет.
func (s *Service) buildConfigMapData(ctx context.Context, configMapId string) (map[string]string, map[string][]byte, error) {
	items, _, err := s.configItemSvc.List(ctx, &configitemModel.ListReq{
		ConfigMapId: new(configMapId),
		Active:      new(true),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("configItemSvc.List: %w", err)
	}

	var data map[string]string
	var binaryData map[string][]byte
	for _, item := range items {
		if errMsgs := validation.IsConfigMapKey(item.Key); len(errMsgs) > 0 {
			return nil, nil, fmt.Errorf("invalid key %q: %s", item.Key, strings.Join(errMsgs, "; "))
		}

		if item.Encoding == "base64" {
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(item.Value))
			if err != nil {
				return nil, nil, fmt.Errorf("key %q: invalid base64 value: %w", item.Key, err)
			}
			if binaryData == nil {
				binaryData = make(map[string][]byte, len(items))
			}
			binaryData[item.Key] = raw
			continue
		}

		if data == nil {
			data = make(map[string]string, len(items))
		}
		data[item.Key] = item.Value
	}

	return data, binaryData, nil
}

func buildConfigMap(want *desiredConfigMap) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      want.name,
			Namespace: want.namespace,
			Labels: map[string]string{
				managedByLabelKey: managedByLabelValue,
			},
			Annotations: map[string]string{
				appIdAnnotation:       want.appId,
				configMapIdAnnotation: want.configMapId,
			},
		},
		Data:       want.data,
		BinaryData: want.binaryData,
	}
}

func configMapUpToDate(current *corev1.ConfigMap, want *desiredConfigMap) bool {
	if current.Annotations[appIdAnnotation] != want.appId ||
		current.Annotations[configMapIdAnnotation] != want.configMapId {
		return false
	}
	if len(current.Data) != len(want.data) {
		return false
	}
	for key, value := range want.data {
		if current.Data[key] != value {
			return false
		}
	}
	if len(current.BinaryData) != len(want.binaryData) {
		return false
	}
	for key, value := range want.binaryData {
		if !bytes.Equal(current.BinaryData[key], value) {
			return false
		}
	}
	return true
}
