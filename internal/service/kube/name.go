package kube

import (
	"unicode/utf8"

	"github.com/rendau/kusec/internal/config"
)

// SecretName — единственная точка формирования имени k8s-секрета:
// используется и при синхронизации в кластер, и для показа в API.
// Префикс (KUBE_SECRET_NAME_PREFIX) отделяет kusec-секреты от существующих
// helm-секретов чартов. При exactSlug=true имя берётся как есть из slug_name,
// без префикса и без app-slug (флаг доступен только админам).
func SecretName(appSlugName, secretSlugName string, exactSlug bool) string {
	if exactSlug {
		return secretSlugName
	}
	return config.Conf.KubeSecretNamePrefix + appSlugName + "-" + secretSlugName
}

// ConfigMapName — единственная точка формирования имени k8s-configmap:
// используется и при синхронизации в кластер, и для показа в API. Префикс
// общий с секретами (KUBE_SECRET_NAME_PREFIX): k8s Secret и ConfigMap — разные
// типы объектов, поэтому одинаковое имя коллизии не вызывает. При exactSlug=true
// имя берётся как есть из slug_name, без префикса и без app-slug.
func ConfigMapName(appSlugName, configMapSlugName string, exactSlug bool) string {
	if exactSlug {
		return configMapSlugName
	}
	return config.Conf.KubeSecretNamePrefix + appSlugName + "-" + configMapSlugName
}

// maxEchoName — сколько символов невалидного имени или ключа показывать в
// тексте ошибки sync.
const maxEchoName = 64

// clipName укорачивает невалидное имя или ключ для текста ошибки. Имя задаёт
// пользователь (в том числе MCP-агент) и может быть любой длины, а ошибки
// sync отдаются в ответах API и MCP: длинный «ключ» со списком вариантов
// превратил бы ошибку в способ подбирать значения секретов (см.
// internal/handler/mcp/errors.go).
func clipName(s string) string {
	if utf8.RuneCountInString(s) <= maxEchoName {
		return s
	}
	return string([]rune(s)[:maxEchoName]) + "…"
}
