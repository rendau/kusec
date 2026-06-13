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

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	"github.com/mechta-market/kusec/internal/errs"
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

// ImportSecrets переносит выбранные секреты кластера в указанное приложение:
// каждый секрет становится записью secret в appId с item-ами по ключам data.
// Уже импортированные (совпал slug в этом приложении) пропускаются. Источник в
// кластере не изменяется. Ошибки отдельных секретов собираются в результат, не
// прерывая импорт остальных.
func (s *Service) ImportSecrets(ctx context.Context, appId string, refs []ImportRef) (*ImportResult, error) {
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

	result := &ImportResult{}

	for _, ref := range refs {
		key := ref.Namespace + "/" + ref.Name

		ksec, err := client.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: get: %v", key, err))
			continue
		}

		// Имя k8s-секрета — валидный DNS1123-subdomain, годится как slug.
		slug := ksec.Name
		if errMsgs := validation.IsDNS1123Subdomain(slug); len(errMsgs) > 0 {
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: invalid secret name: %s", key, strings.Join(errMsgs, "; ")))
			continue
		}

		existing, err := s.findSecretBySlug(ctx, app.Id, slug)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", key, err))
			continue
		}
		if existing != nil {
			result.Skipped = append(result.Skipped, key)
			continue
		}

		secretId, err := s.secretSvc.Create(ctx, &secretModel.Edit{
			AppId:       lo.ToPtr(app.Id),
			Active:      lo.ToPtr(true),
			SlugName:    lo.ToPtr(slug),
			Description: lo.ToPtr("Imported from " + key),
			KubeType:    lo.ToPtr(displaySecretType(ksec.Type)),
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: create secret: %v", key, err))
			continue
		}
		result.CreatedSecrets++

		for _, dataKey := range sortedDataKeys(ksec.Data) {
			value, encoding := encodeImportValue(ksec.Data[dataKey])
			_, err = s.itemSvc.Create(ctx, &itemModel.Edit{
				SecretId: lo.ToPtr(secretId),
				Active:   lo.ToPtr(true),
				Key:      lo.ToPtr(dataKey),
				Value:    lo.ToPtr(value),
				Encoding: lo.ToPtr(encoding),
			})
			if err != nil {
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: key %q: create item: %v", key, dataKey, err))
				continue
			}
			result.CreatedItems++
		}

		result.Imported = append(result.Imported, key)
	}

	sort.Strings(result.Imported)
	sort.Strings(result.Skipped)
	sort.Strings(result.Errors)

	return result, nil
}

func (s *Service) findSecretBySlug(ctx context.Context, appId, slug string) (*secretModel.Main, error) {
	secrets, _, err := s.secretSvc.List(ctx, &secretModel.ListReq{AppId: lo.ToPtr(appId)})
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
