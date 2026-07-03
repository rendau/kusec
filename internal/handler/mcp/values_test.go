package mcp

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateValue_Formats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		format  string
		length  int
		charset string
	}{
		{"", 32, alnumChars},
		{"alnum", 10, alnumChars},
		{"ascii", 64, alnumChars + asciiExtras},
		{"digits", 6, digitChars},
		{"hex", 33, "0123456789abcdef"},
		{"base64url", 43, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"},
	}

	for _, tc := range cases {
		t.Run("format_"+tc.format, func(t *testing.T) {
			t.Parallel()

			value, err := generateValue(tc.format, tc.length)
			require.NoError(t, err)
			require.Len(t, value, tc.length)

			for _, r := range value {
				assert.Contains(t, tc.charset, string(r))
			}
		})
	}
}

func TestGenerateValue_DefaultLength(t *testing.T) {
	t.Parallel()

	value, err := generateValue("", 0)
	require.NoError(t, err)
	assert.Len(t, value, defaultGenLength)
}

func TestGenerateValue_Uuid(t *testing.T) {
	t.Parallel()

	value, err := generateValue("uuid", 0)
	require.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`), value)
}

func TestGenerateValue_Errors(t *testing.T) {
	t.Parallel()

	_, err := generateValue("unknown", 10)
	require.Error(t, err)

	_, err = generateValue("", maxGenLength+1)
	require.Error(t, err)
}

func TestGenerateValue_Unique(t *testing.T) {
	t.Parallel()

	a, err := generateValue("", 32)
	require.NoError(t, err)
	b, err := generateValue("", 32)
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
}

func TestMaskValue(t *testing.T) {
	t.Parallel()

	masked := maskValue("пароль")

	assert.Equal(t, 6, masked.Chars)
	assert.Equal(t, 12, masked.Bytes)
	assert.Len(t, masked.Sha256, valueHashLen)

	// детерминированность и различимость
	assert.Equal(t, masked.Sha256, maskValue("пароль").Sha256)
	assert.NotEqual(t, masked.Sha256, maskValue("другой").Sha256)
}

func TestVault_RememberLookup(t *testing.T) {
	t.Parallel()

	v := newVault()
	v.remember("app1", "db_password", "secret-value-1")

	got, ok := v.lookup("app1", "db_password")
	require.True(t, ok)
	assert.Equal(t, "secret-value-1", got)

	// реестр изолирован по app
	_, ok = v.lookup("app2", "db_password")
	assert.False(t, ok)

	assert.Equal(t, []string{"db_password"}, v.names("app1"))
	assert.Empty(t, v.names("app2"))
}

func TestVault_Scrub(t *testing.T) {
	t.Parallel()

	v := newVault()
	v.remember("app1", "token", "sUp3r-s3cr3t")
	v.markSeen("another-secret")
	v.markSeen("")

	scrubbed := v.scrub(`error: value "sUp3r-s3cr3t" conflicts with another-secret`)

	assert.NotContains(t, scrubbed, "sUp3r-s3cr3t")
	assert.NotContains(t, scrubbed, "another-secret")
	assert.True(t, strings.Contains(scrubbed, "[REDACTED]"))

	// текст без секретов не меняется
	assert.Equal(t, "обычная ошибка", v.scrub("обычная ошибка"))
}
