package kube

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

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

func TestClipName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "billing-db", clipName("billing-db"))

	long := strings.Repeat("482913.", 40)
	clipped := clipName(long)
	assert.Equal(t, maxEchoName+1, utf8.RuneCountInString(clipped))
	assert.True(t, strings.HasSuffix(clipped, "…"))
}
