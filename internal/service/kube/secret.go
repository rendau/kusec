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
	"k8s.io/client-go/kubernetes"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	"github.com/mechta-market/kusec/internal/errs"
)

// SyncSecrets приводит k8s-секреты в соответствие с базой:
// создаёт новые, обновляет изменившиеся, удаляет лишние.
// Управляются только секреты с лейблом app.kubernetes.io/managed-by=kusec —
// чужие секреты кластера не трогаются. Ошибки отдельных секретов собираются
// в результат, не прерывая синхронизацию остальных.
func (s *Service) SyncSecrets(ctx context.Context, appIds []string) (*SyncResult, error) {
	if !s.mu.TryLock() {
		return nil, errs.SyncInProgress
	}
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	client, err := s.getClient()
	if err != nil {
		return nil, err
	}

	return s.syncSecretsLocked(ctx, client, appIds)
}

// syncSecretsLocked выполняет реконсиляцию секретов; вызывается под s.mu.
func (s *Service) syncSecretsLocked(ctx context.Context, client kubernetes.Interface, appIds []string) (*SyncResult, error) {
	result := &SyncResult{}

	desired, err := s.buildDesired(ctx, result, appIds)
	if err != nil {
		return nil, err
	}

	existingList, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: managedBySelector,
	})
	if err != nil {
		return nil, fmt.Errorf("k8s: list managed secrets: %w", err)
	}

	// Пустой appIds означает «все приложения» (симметрично buildDesired с
	// Ids: nil): фильтровать существующие по scope в этом случае не нужно,
	// иначе existing окажется пустым и всё уйдёт в ошибочный повторный Create.
	all := len(appIds) == 0
	scopeSet := lo.SliceToMap(appIds, func(appId string) (string, bool) { return appId, true })

	existing := make(map[string]*corev1.Secret, len(existingList.Items))
	for i := range existingList.Items {
		secret := &existingList.Items[i]
		if !all && !scopeSet[secret.Annotations[appIdAnnotation]] {
			continue
		}
		existing[secret.Namespace+"/"+secret.Name] = secret
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
			_, err = client.CoreV1().Secrets(want.namespace).Create(ctx, buildSecret(want), metav1.CreateOptions{})
			if err == nil {
				result.Created = append(result.Created, key)
				continue
			}
			if !k8serrors.IsAlreadyExists(err) {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: create: %v", key, err))
				continue
			}
			// Секрет уже есть, но без нашего лейбла (создан вне kusec/старой
			// версией) — усыновляем: подтягиваем текущий и обновляем.
			current, err = client.CoreV1().Secrets(want.namespace).Get(ctx, want.name, metav1.GetOptions{})
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: adopt: %v", key, err))
				continue
			}
		}

		s.reconcileExistingSecret(ctx, client, current, want, key, result)
	}

	// Управляемые секреты, которым больше нет активных записей в базе.
	for key, stale := range existing {
		err = client.CoreV1().Secrets(stale.Namespace).Delete(ctx, stale.Name, metav1.DeleteOptions{})
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

// reconcileExistingSecret приводит существующий секрет к желаемому состоянию.
// Используется и для найденных по лейблу, и для усыновляемых (Create →
// AlreadyExists), поэтому всегда проставляет managed-by лейбл и аннотации.
func (s *Service) reconcileExistingSecret(
	ctx context.Context,
	client kubernetes.Interface,
	current *corev1.Secret,
	want *desiredSecret,
	key string,
	result *SyncResult,
) {
	if secretUpToDate(current, want) {
		result.Unchanged++
		return
	}

	// Тип k8s-секрета immutable: при его смене секрет пересоздаётся.
	if current.Type != desiredSecretType(want) {
		if err := client.CoreV1().Secrets(want.namespace).Delete(ctx, want.name, metav1.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: recreate (delete): %v", key, err))
			return
		}
		if _, err := client.CoreV1().Secrets(want.namespace).Create(ctx, buildSecret(want), metav1.CreateOptions{}); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: recreate (create): %v", key, err))
			return
		}
		result.Updated = append(result.Updated, key)
		return
	}

	updated := current.DeepCopy()
	updated.Data = want.data
	updated.StringData = nil
	if updated.Labels == nil {
		updated.Labels = map[string]string{}
	}
	updated.Labels[managedByLabelKey] = managedByLabelValue
	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}
	updated.Annotations[appIdAnnotation] = want.appId
	updated.Annotations[secretIdAnnotation] = want.secretId

	if _, err := client.CoreV1().Secrets(want.namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("%s: update: %v", key, err))
		return
	}
	result.Updated = append(result.Updated, key)
}

// buildDesired собирает желаемое состояние из базы: только active-записи
// (app, secret, item). Невалидные секреты пропускаются с ошибкой в result.
func (s *Service) buildDesired(ctx context.Context, result *SyncResult, appIds []string) (map[string]*desiredSecret, error) {
	apps, _, err := s.appSvc.List(ctx, &appModel.ListReq{Ids: appIds, Active: new(true)})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}

	desired := make(map[string]*desiredSecret)

	for _, app := range apps {
		if errMsgs := validation.IsDNS1123Label(app.Namespace); len(errMsgs) > 0 {
			result.Errors = append(result.Errors,
				fmt.Sprintf("app %q: invalid namespace %q: %s", app.SlugName, app.Namespace, strings.Join(errMsgs, "; ")))
			continue
		}

		secrets, _, err := s.secretSvc.List(ctx, &secretModel.ListReq{
			AppId:  new(app.Id),
			Active: new(true),
		})
		if err != nil {
			return nil, fmt.Errorf("secretSvc.List: %w", err)
		}

		for _, secret := range secrets {
			name := SecretName(app.SlugName, secret.SlugName)
			key := app.Namespace + "/" + name

			if errMsgs := validation.IsDNS1123Subdomain(name); len(errMsgs) > 0 {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: invalid secret name: %s", key, strings.Join(errMsgs, "; ")))
				continue
			}
			if clash, ok := desired[key]; ok {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: name collision between app ids %s and %s", key, clash.appId, app.Id))
				continue
			}

			data, err := s.buildSecretData(ctx, secret.Id)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", key, err))
				continue
			}

			desired[key] = &desiredSecret{
				namespace: app.Namespace,
				name:      name,
				appId:     app.Id,
				secretId:  secret.Id,
				kubeType:  secret.KubeType,
				data:      data,
			}
		}
	}

	return desired, nil
}

// buildSecretData собирает data секрета из active-items.
// encoding=base64 — значение хранится в base64 (файлы) и декодируется в байты;
// иначе значение кладётся как есть (k8s сам кодирует Data в base64 при
// сериализации). value_format (text/yaml/json) — подсказка редактора и на
// содержимое не влияет.
func (s *Service) buildSecretData(ctx context.Context, secretId string) (map[string][]byte, error) {
	items, _, err := s.itemSvc.List(ctx, &itemModel.ListReq{
		SecretId: new(secretId),
		Active:   new(true),
	})
	if err != nil {
		return nil, fmt.Errorf("itemSvc.List: %w", err)
	}

	data := make(map[string][]byte, len(items))
	for _, item := range items {
		if errMsgs := validation.IsConfigMapKey(item.Key); len(errMsgs) > 0 {
			return nil, fmt.Errorf("invalid key %q: %s", item.Key, strings.Join(errMsgs, "; "))
		}

		if item.Encoding == "base64" {
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(item.Value))
			if err != nil {
				return nil, fmt.Errorf("key %q: invalid base64 value: %w", item.Key, err)
			}
			data[item.Key] = raw
			continue
		}

		data[item.Key] = []byte(item.Value)
	}

	return data, nil
}

// desiredSecretType — тип k8s-секрета из записи базы; пусто = Opaque.
func desiredSecretType(want *desiredSecret) corev1.SecretType {
	if want.kubeType == "" {
		return corev1.SecretTypeOpaque
	}
	return corev1.SecretType(want.kubeType)
}

func buildSecret(want *desiredSecret) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      want.name,
			Namespace: want.namespace,
			Labels: map[string]string{
				managedByLabelKey: managedByLabelValue,
			},
			Annotations: map[string]string{
				appIdAnnotation:    want.appId,
				secretIdAnnotation: want.secretId,
			},
		},
		Type: desiredSecretType(want),
		Data: want.data,
	}
}

func secretUpToDate(current *corev1.Secret, want *desiredSecret) bool {
	if current.Type != desiredSecretType(want) {
		return false
	}
	if current.Annotations[appIdAnnotation] != want.appId ||
		current.Annotations[secretIdAnnotation] != want.secretId {
		return false
	}
	if len(current.Data) != len(want.data) {
		return false
	}
	for key, value := range want.data {
		if !bytes.Equal(current.Data[key], value) {
			return false
		}
	}
	return true
}
