package kube

import "github.com/mechta-market/kusec/internal/config"

// SecretName — единственная точка формирования имени k8s-секрета:
// используется и при синхронизации в кластер, и для показа в API.
// Префикс (KUBE_SECRET_NAME_PREFIX) отделяет kusec-секреты от существующих
// helm-секретов чартов.
func SecretName(appSlugName, secretSlugName string) string {
	return config.Conf.KubeSecretNamePrefix + appSlugName + "-" + secretSlugName
}
