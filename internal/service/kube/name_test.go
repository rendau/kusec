package kube

import (
	"testing"

	"github.com/rendau/kusec/internal/config"
)

func TestSecretName_UsesConfiguredPrefix(t *testing.T) {
	t.Parallel()

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
