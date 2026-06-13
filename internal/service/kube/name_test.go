package kube

import (
	"testing"

	"github.com/mechta-market/kusec/internal/config"
)

func TestSecretName_UsesConfiguredPrefix(t *testing.T) {
	t.Parallel()

	originalPrefix := config.Conf.KubeSecretNamePrefix
	config.Conf.KubeSecretNamePrefix = "pref-"
	t.Cleanup(func() {
		config.Conf.KubeSecretNamePrefix = originalPrefix
	})

	got := SecretName("orders", "db")
	if got != "pref-orders-db" {
		t.Fatalf("SecretName() = %q, want %q", got, "pref-orders-db")
	}
}
