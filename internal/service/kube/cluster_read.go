package kube

import (
	"context"
	"errors"
	"fmt"
	"sort"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rendau/kusec/internal/errs"
)

// sortedConfigMapKeys возвращает объединённые отсортированные ключи текстовых
// (Data) и бинарных (BinaryData) значений configmap. Ключи в k8s configmap
// уникальны между двумя картами, поэтому простое слияние без дублей.
func sortedConfigMapKeys(data map[string]string, binaryData map[string][]byte) []string {
	keys := make([]string, 0, len(data)+len(binaryData))
	for key := range data {
		keys = append(keys, key)
	}
	for key := range binaryData {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// GetClusterSecret читает живой k8s-secret по namespace/name для сверки с
// записью kusec. Возвращает (resource, inCluster, found, err): вне кластера —
// inCluster=false (штатно, без ошибки); отсутствие объекта — found=false.
// Значения отдаются (текст или base64 для бинарных) — зеркально импорту.
func (s *Service) GetClusterSecret(ctx context.Context, namespace, name string) (*ClusterResource, bool, bool, error) {
	client, err := s.getClient()
	if err != nil {
		if errors.Is(err, errs.NotInCluster) {
			return nil, false, false, nil
		}
		return nil, false, false, err
	}

	sec, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, true, false, nil
	}
	if err != nil {
		return nil, true, false, fmt.Errorf("k8s: get secret %s/%s: %w", namespace, name, err)
	}

	res := &ClusterResource{
		Namespace: sec.Namespace,
		Name:      sec.Name,
		Type:      displaySecretType(sec.Type),
		Managed:   sec.Labels[managedByLabelKey] == managedByLabelValue,
	}
	for _, key := range sortedDataKeys(sec.Data) {
		value, encoding := encodeImportValue(sec.Data[key])
		res.Items = append(res.Items, ClusterResourceItem{Key: key, Value: value, Encoding: encoding})
	}

	return res, true, true, nil
}

// GetClusterConfigMap читает живой k8s-configmap по namespace/name для сверки.
// Текстовые значения (Data) отдаются как plain, бинарные (BinaryData) — как
// base64. Семантика возврата — как у GetClusterSecret.
func (s *Service) GetClusterConfigMap(ctx context.Context, namespace, name string) (*ClusterResource, bool, bool, error) {
	client, err := s.getClient()
	if err != nil {
		if errors.Is(err, errs.NotInCluster) {
			return nil, false, false, nil
		}
		return nil, false, false, err
	}

	cm, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, true, false, nil
	}
	if err != nil {
		return nil, true, false, fmt.Errorf("k8s: get configmap %s/%s: %w", namespace, name, err)
	}

	res := &ClusterResource{
		Namespace: cm.Namespace,
		Name:      cm.Name,
		Managed:   cm.Labels[managedByLabelKey] == managedByLabelValue,
	}
	for _, key := range sortedConfigMapKeys(cm.Data, cm.BinaryData) {
		if value, ok := cm.Data[key]; ok {
			res.Items = append(res.Items, ClusterResourceItem{Key: key, Value: value, Encoding: "plain"})
			continue
		}
		value, encoding := encodeImportValue(cm.BinaryData[key])
		res.Items = append(res.Items, ClusterResourceItem{Key: key, Value: value, Encoding: encoding})
	}

	return res, true, true, nil
}
