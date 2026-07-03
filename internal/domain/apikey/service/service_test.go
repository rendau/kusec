package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mechta-market/kusec/internal/constant"
)

func TestGenerateKey(t *testing.T) {
	t.Parallel()

	key, hash, prefix, err := GenerateKey()
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(key, constant.ApiKeyPrefix))
	assert.Len(t, key, len(constant.ApiKeyPrefix)+keyRandomBytes*2)

	assert.Equal(t, HashKey(key), hash)
	assert.NotEqual(t, key, hash)

	assert.Equal(t, key[:keyPrefixLen], prefix)
	assert.True(t, strings.HasPrefix(key, prefix))

	// ключи уникальны
	key2, _, _, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2)
}

func TestHashKey_Deterministic(t *testing.T) {
	t.Parallel()

	assert.Equal(t, HashKey("ksk_abc"), HashKey("ksk_abc"))
	assert.NotEqual(t, HashKey("ksk_abc"), HashKey("ksk_abd"))
	assert.Len(t, HashKey("ksk_abc"), 64)
}
