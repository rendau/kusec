package kube

// SecretName — единственная точка формирования имени k8s-секрета:
// используется и при синхронизации в кластер, и для показа в API.
func SecretName(appSlugName, secretSlugName string) string {
	return appSlugName + "-" + secretSlugName
}
