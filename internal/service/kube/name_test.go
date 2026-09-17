package kube

import (
	"testing"

	"github.com/rendau/kusec/internal/config"
)

// Не t.Parallel(): тест мутирует глобальный config.Conf, и параллельный
// запуск утёк бы префиксом в соседние тесты пакета.
func TestSecretName_UsesConfiguredPrefix(t *testing.T) {
	originalPrefix := config.Conf.KubeSecretNamePrefix
	config.Conf.KubeSecretNamePrefix = "pref-"
	t.Cleanup(func() {
		config.Conf.KubeSecretNamePrefix = originalPrefix
	})

	got := SecretName("orders", "db", false)
	if got != "pref-orders-db" {
		t.Fatalf("SecretName() = %q, want %q", got, "pref-orders-db")
	}
}

func TestSecretName_ExactSlugDropsPrefix(t *testing.T) {
	t.Parallel()

	// exactSlug=true: префикс не применяется, имя == slug_name как есть.
	got := SecretName("orders", "db", true)
	if got != "db" {
		t.Fatalf("SecretName() = %q, want %q", got, "db")
	}
}
