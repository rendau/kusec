package secretbox

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testScope = "item.value"

// testKey — ключ из 32 одинаковых байт; разные fill дают разные ключи.
func testKey(fill byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, KeySize))
}

func newTestBox(t *testing.T, fill byte) *Box {
	t.Helper()
	box, err := New(testKey(fill))
	require.NoError(t, err)
	require.True(t, box.Enabled())
	return box
}

func TestNewKeyFormat(t *testing.T) {
	// пустой ключ — шифрование выключено, это не ошибка
	for _, key := range []string{"", "   ", "\n"} {
		box, err := New(key)
		require.NoError(t, err, "key=%q", key)
		assert.False(t, box.Enabled(), "key=%q", key)
	}

	// пробелы и перевод строки вокруг ключа не мешают (значение из файла/секрета)
	box, err := New("  " + testKey(1) + "\n")
	require.NoError(t, err)
	assert.True(t, box.Enabled())

	invalid := map[string]string{
		"not base64":      "this is not base64!!!",
		"16 bytes":        base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16)),
		"31 bytes":        base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 31)),
		"33 bytes":        base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 33)),
		"url-safe base64": base64.URLEncoding.EncodeToString(bytes.Repeat([]byte{0xff}, KeySize)),
		"no padding":      base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{1}, KeySize)),
	}
	for name, key := range invalid {
		box, err = New(key)
		require.ErrorIs(t, err, ErrInvalidKey, name)
		assert.Nil(t, box, name)
		// ключ не утекает в текст ошибки
		assert.NotContains(t, err.Error(), key, name)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	box := newTestBox(t, 1)

	values := []string{
		"s3cret",
		"пароль с юникодом ✓",
		"line1\nline2\n",
		"enc:v1:выглядит как шифртекст, но это открытый текст",
		strings.Repeat("x", 100_000),
		string([]byte{0, 1, 2, 0xff, 0xfe}),
	}
	for _, plain := range values {
		sealed, err := box.Seal(testScope, plain)
		require.NoError(t, err)

		assert.True(t, IsSealed(sealed))
		assert.True(t, strings.HasPrefix(sealed, Prefix))
		if len(plain) > 8 {
			assert.NotContains(t, sealed, plain)
		}
		// после префикса — base64 от nonce(12) ‖ ciphertext ‖ tag(16)
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, Prefix))
		require.NoError(t, err)
		assert.Len(t, raw, 12+len(plain)+16)

		opened, err := box.Open(testScope, sealed)
		require.NoError(t, err)
		assert.Equal(t, plain, opened)
	}
}

// Каждое шифрование — со своим nonce: одинаковые значения дают разные строки,
// по шифртексту не видно, что значения совпадают.
func TestSealIsRandomized(t *testing.T) {
	box := newTestBox(t, 1)

	first, err := box.Seal(testScope, "same-value")
	require.NoError(t, err)
	second, err := box.Seal(testScope, "same-value")
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

// Пустое значение не шифруется: проверки на пустоту в SQL продолжают работать.
func TestEmptyValueStaysEmpty(t *testing.T) {
	box := newTestBox(t, 1)

	sealed, err := box.Seal(testScope, "")
	require.NoError(t, err)
	assert.Empty(t, sealed)
	assert.False(t, IsSealed(sealed))

	opened, err := box.Open(testScope, "")
	require.NoError(t, err)
	assert.Empty(t, opened)
}

// Строка без префикса — ещё не зашифрованная старая строка: читается как есть.
func TestOpenPlainLegacyValue(t *testing.T) {
	box := newTestBox(t, 1)

	for _, legacy := range []string{"legacy-plain-value", "enc:v2:другая версия", "enc:v1", "ENC:V1:upper"} {
		opened, err := box.Open(testScope, legacy)
		require.NoError(t, err)
		assert.Equal(t, legacy, opened)
	}
}

func TestOpenWithWrongKey(t *testing.T) {
	sealed, err := newTestBox(t, 1).Seal(testScope, "s3cret")
	require.NoError(t, err)

	opened, err := newTestBox(t, 2).Open(testScope, sealed)
	require.ErrorIs(t, err, ErrDecrypt)
	assert.Empty(t, opened)
}

// scope входит в AAD: шифртекст, переставленный в другую колонку, не
// расшифровывается.
func TestOpenWithWrongScope(t *testing.T) {
	box := newTestBox(t, 1)

	sealed, err := box.Seal("item.value", "s3cret")
	require.NoError(t, err)

	opened, err := box.Open("usr.totp_secret", sealed)
	require.ErrorIs(t, err, ErrDecrypt)
	assert.Empty(t, opened)
}

func TestOpenCorruptedValue(t *testing.T) {
	box := newTestBox(t, 1)

	sealed, err := box.Seal(testScope, "s3cret-value")
	require.NoError(t, err)

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sealed, Prefix))
	require.NoError(t, err)

	flipped := bytes.Clone(raw)
	flipped[len(flipped)-1] ^= 0x01

	corrupted := map[string]string{
		"flipped bit":    Prefix + base64.StdEncoding.EncodeToString(flipped),
		"truncated":      Prefix + base64.StdEncoding.EncodeToString(raw[:len(raw)-4]),
		"too short":      Prefix + base64.StdEncoding.EncodeToString(raw[:10]),
		"not base64":     Prefix + "%%% not base64 %%%",
		"only prefix":    Prefix,
		"plain w/prefix": Prefix + "просто текст",
	}
	for name, stored := range corrupted {
		opened, err := box.Open(testScope, stored)
		require.ErrorIs(t, err, ErrDecrypt, name)
		assert.Empty(t, opened, name)
	}
}

// Выключенный режим: значения проходят как есть, поведение сервиса не меняется.
func TestDisabledModePassThrough(t *testing.T) {
	disabled, err := New("")
	require.NoError(t, err)

	var nilBox *Box

	for name, box := range map[string]*Box{"empty key": disabled, "nil": nilBox} {
		assert.False(t, box.Enabled(), name)

		for _, plain := range []string{"", "plain-value", "значение"} {
			sealed, err := box.Seal(testScope, plain)
			require.NoError(t, err, name)
			assert.Equal(t, plain, sealed, name)

			opened, err := box.Open(testScope, plain)
			require.NoError(t, err, name)
			assert.Equal(t, plain, opened, name)
		}
	}
}

// Зашифрованная строка без ключа — ошибка, а не шифртекст вместо значения.
func TestDisabledModeRejectsSealedValue(t *testing.T) {
	sealed, err := newTestBox(t, 1).Seal(testScope, "s3cret")
	require.NoError(t, err)

	disabled, err := New("")
	require.NoError(t, err)
	var nilBox *Box

	for name, box := range map[string]*Box{"empty key": disabled, "nil": nilBox} {
		opened, err := box.Open(testScope, sealed)
		require.ErrorIs(t, err, ErrKeyRequired, name)
		assert.Empty(t, opened, name)
		assert.Contains(t, err.Error(), "DATA_ENCRYPTION_KEY", name)

		// открытый текст с зарезервированным префиксом без ключа не сохранить:
		// он выглядел бы зашифрованным
		stored, err := box.Seal(testScope, Prefix+"anything")
		require.ErrorIs(t, err, ErrReservedPrefix, name)
		assert.Empty(t, stored, name)
	}
}

func TestScopeIsRequired(t *testing.T) {
	disabled, err := New("")
	require.NoError(t, err)

	for name, box := range map[string]*Box{"enabled": newTestBox(t, 1), "disabled": disabled} {
		_, err = box.Seal("", "value")
		require.ErrorIs(t, err, ErrScopeRequired, name)
		_, err = box.Open("", "value")
		require.ErrorIs(t, err, ErrScopeRequired, name)
	}
}

func TestIsSealed(t *testing.T) {
	assert.True(t, IsSealed(Prefix+"abc"))
	assert.True(t, IsSealed(Prefix))
	assert.False(t, IsSealed(""))
	assert.False(t, IsSealed("plain"))
	assert.False(t, IsSealed(" "+Prefix+"abc"))
	assert.False(t, IsSealed("enc:v2:abc"))
}
