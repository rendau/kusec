package kube

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
)

// importSkippedTypes — типы секретов, не предназначенные для импорта:
// служебные токены ServiceAccount и хранилища релизов helm.
var importSkippedTypes = map[corev1.SecretType]bool{
	corev1.SecretTypeServiceAccountToken: true,
	"helm.sh/release.v1":                 true,
}

// ListClusterSecrets возвращает секреты кластера для выбора при импорте:
// без системных namespace-ов (kube-*) и без служебных типов (токены SA,
// helm-релизы). Значения не отдаются — только ключи data. Вне кластера это
// штатная ситуация: inCluster=false и пустой список без ошибки.
func (s *Service) ListClusterSecrets(ctx context.Context, namespace string) ([]*ClusterSecret, bool, error) {
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

	list, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("k8s: list secrets: %w", err)
	}

	result := make([]*ClusterSecret, 0, len(list.Items))
	for i := range list.Items {
		sec := &list.Items[i]
		if systemNamespaces[sec.Namespace] || importSkippedTypes[sec.Type] {
			continue
		}

		result = append(result, &ClusterSecret{
			Namespace: sec.Namespace,
			Name:      sec.Name,
			Type:      displaySecretType(sec.Type),
			Keys:      sortedDataKeys(sec.Data),
			Managed:   sec.Labels[managedByLabelKey] == managedByLabelValue,
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

// ImportSecret переносит один секрет кластера в указанное приложение:
// секрет становится записью secret в appId с item-ами по ключам data.
// secretSlug задаёт имя посадочного секрета (обязателен, валидируется в
// usecase). Если секрет с таким slug в приложении уже есть — выполняется
// дозаполнение: недостающие ключи создаются, совпавшие — перезаписываются
// (значение из кластера). Источник в кластере не изменяется.
func (s *Service) ImportSecret(ctx context.Context, appId string, ref ImportRef, secretSlug string) (*ImportResult, error) {
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}

	// Проверяем существование целевого приложения. errNE → ObjectNotFound
	// возвращаем как есть (сентинель), чтобы usecase пробросил его клиенту.
	app, _, err := s.appSvc.Get(ctx, appId, true)
	if err != nil {
		return nil, err
	}

	ksec, err := client.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get cluster secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	// Slug посадочного секрета обязан быть валидным DNS1123-subdomain (имя
	// k8s-секрета).
	slug := strings.TrimSpace(secretSlug)
	if errMsgs := validation.IsDNS1123Subdomain(slug); len(errMsgs) > 0 {
		return nil, errs.ErrFull{Err: errs.InvalidRequest, Desc: "invalid secret slug: " + strings.Join(errMsgs, "; ")}
	}

	existing, err := s.findSecretBySlug(ctx, app.Id, slug)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{SecretSlug: slug}

	// Существующий секрет переиспользуем (дозаполнение), иначе создаём новый.
	// existingItems: ключ → id item-а (active и неактивные — чтобы найти любой
	// совпавший ключ и не плодить дубли).
	existingItems := map[string]string{}
	if existing != nil {
		result.SecretId = existing.Id
		items, _, err := s.itemSvc.List(ctx, &itemModel.ListReq{SecretId: new(existing.Id)})
		if err != nil {
			return nil, fmt.Errorf("list existing items: %w", err)
		}
		for _, it := range items {
			existingItems[it.Key] = it.Id
		}
	} else {
		secretId, err := s.secretSvc.Create(ctx, &secretModel.Edit{
			AppId:       new(app.Id),
			Active:      new(true),
			SlugName:    new(slug),
			Description: new(fmt.Sprintf("Imported from %s/%s", ref.Namespace, ref.Name)),
			KubeType:    new(displaySecretType(ksec.Type)),
		})
		if err != nil {
			return nil, fmt.Errorf("create secret: %w", err)
		}
		result.SecretId = secretId
		result.SecretCreated = true
	}

	for _, dataKey := range sortedDataKeys(ksec.Data) {
		value, encoding := encodeImportValue(ksec.Data[dataKey])

		// Совпавший ключ — перезаписываем значение существующего item-а.
		if itemId, ok := existingItems[dataKey]; ok {
			err = s.itemSvc.Update(ctx, itemId, &itemModel.Edit{
				Value:    new(value),
				Encoding: new(encoding),
			})
			if err != nil {
				return nil, fmt.Errorf("key %q: update item: %w", dataKey, err)
			}
			result.UpdatedItems++
			continue
		}

		_, err = s.itemSvc.Create(ctx, &itemModel.Edit{
			SecretId: new(result.SecretId),
			Active:   new(true),
			Key:      new(dataKey),
			Value:    new(value),
			Encoding: new(encoding),
		})
		if err != nil {
			return nil, fmt.Errorf("key %q: create item: %w", dataKey, err)
		}
		result.CreatedItems++
	}

	return result, nil
}

func (s *Service) findSecretBySlug(ctx context.Context, appId, slug string) (*secretModel.Main, error) {
	secrets, _, err := s.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(appId)})
	if err != nil {
		return nil, fmt.Errorf("secretSvc.List: %w", err)
	}
	for _, secret := range secrets {
		if secret.SlugName == slug {
			return secret, nil
		}
	}
	return nil, nil
}

// displaySecretType приводит тип k8s-секрета к виду базы: Opaque хранится как
// пустая строка.
func displaySecretType(t corev1.SecretType) string {
	if t == corev1.SecretTypeOpaque {
		return ""
	}
	return string(t)
}

func sortedDataKeys(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// encodeImportValue выбирает способ хранения значения: текст как есть
// (encoding=plain) либо base64 для бинарных данных (encoding=base64).
// Зеркально buildSecretData, которая раскодирует base64 обратно при sync.
func encodeImportValue(raw []byte) (value string, encoding string) {
	if utf8.Valid(raw) && !bytes.ContainsRune(raw, 0) {
		return string(raw), "plain"
	}
	return base64.StdEncoding.EncodeToString(raw), "base64"
}
