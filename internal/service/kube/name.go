package kube

import "github.com/mechta-market/kusec/internal/config"

// SecretName — единственная точка формирования имени k8s-секрета:
// используется и при синхронизации в кластер, и для показа в API.
// Префикс (KUBE_SECRET_NAME_PREFIX) отделяет kusec-секреты от существующих
// helm-секретов чартов.
func SecretName(appSlugName, secretSlugName string) string {
	return config.Conf.KubeSecretNamePrefix + appSlugName + "-" + secretSlugName
}

// ConfigMapName — единственная точка формирования имени k8s-configmap:
// используется и при синхронизации в кластер, и для показа в API. Префикс
// общий с секретами (KUBE_SECRET_NAME_PREFIX): k8s Secret и ConfigMap — разные
// типы объектов, поэтому одинаковое имя коллизии не вызывает.
func ConfigMapName(appSlugName, configMapSlugName string) string {
	return config.Conf.KubeSecretNamePrefix + appSlugName + "-" + configMapSlugName
}
