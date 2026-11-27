package service

import (
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret"

func signTestClaims(t *testing.T, secret string, claims jwtv5.MapClaims) string {
	t.Helper()
	token, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return token
}

// Access-токен несёт id пользователя и номер поколения сессий — и никаких прав.
func TestAccessToken_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := New(testSecret)

	token, err := svc.CreateToken(42, 7)
	require.NoError(t, err)

	usrId, ver, err := svc.ParseAccessToken(token)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)
	assert.EqualValues(t, 7, ver)

	claims := jwtv5.MapClaims{}
	_, _, err = jwtv5.NewParser().ParseUnverified(token, claims)
	require.NoError(t, err)
	assert.NotContains(t, claims, "is_admin")
	assert.NotContains(t, claims, "app_ids")
	assert.Equal(t, tokenTypeAccess, claims["typ"])

	// чужая подпись и мусор
	_, _, err = New("other-secret").ParseAccessToken(token)
	require.Error(t, err)
	_, _, err = svc.ParseAccessToken("garbage")
	require.Error(t, err)
}

// Refresh-токен несёт тот же номер поколения; как access он не принимается,
// и наоборот.
func TestRefreshToken_RoundTrip(t *testing.T) {
	t.Parallel()
	svc := New(testSecret)

	const passwordHash = "$2a$10$hash"

	refresh, err := svc.CreateRefreshToken(42, passwordHash, 3)
	require.NoError(t, err)

	usrId, err := svc.RefreshTokenUserId(refresh)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)

	usrId, ver, err := svc.ParseRefreshToken(refresh, passwordHash)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)
	assert.EqualValues(t, 3, ver)

	// смена пароля отзывает refresh-токен
	_, _, err = svc.ParseRefreshToken(refresh, "$2a$10$other")
	require.Error(t, err)

	// типы токенов не взаимозаменяемы
	_, _, err = svc.ParseAccessToken(refresh)
	require.Error(t, err)

	access, err := svc.CreateToken(42, 3)
	require.NoError(t, err)
	_, _, err = svc.ParseRefreshToken(access, passwordHash)
	require.Error(t, err)
	_, err = svc.RefreshTokenUserId(access)
	require.Error(t, err)
}

// Токены, выданные до появления claim ver (и до claim typ), — поколение 0.
func TestLegacyTokens_VersionZero(t *testing.T) {
	t.Parallel()
	svc := New(testSecret)

	now := time.Now().UTC()
	exp := now.Add(time.Minute).Unix()

	for name, claims := range map[string]jwtv5.MapClaims{
		"со старыми правами": {"typ": tokenTypeAccess, "id": 42, "is_admin": true, "app_ids": []string{"a"}, "exp": exp},
		"без typ":            {"id": 42, "is_admin": false, "exp": exp},
		"id строкой":         {"typ": tokenTypeAccess, "id": "42", "exp": exp},
	} {
		usrId, ver, err := svc.ParseAccessToken(signTestClaims(t, testSecret, claims))
		require.NoError(t, err, name)
		assert.EqualValues(t, 42, usrId, name)
		assert.EqualValues(t, 0, ver, name)
	}

	const passwordHash = "$2a$10$hash"
	legacyRefresh := signTestClaims(t, testSecret, jwtv5.MapClaims{
		"typ": tokenTypeRefresh, "id": 42, "pwd": passwordFingerprint(passwordHash), "exp": exp,
	})
	usrId, ver, err := svc.ParseRefreshToken(legacyRefresh, passwordHash)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)
	assert.EqualValues(t, 0, ver)
}

// Битые и просроченные токены отвергаются.
func TestParseAccessToken_Rejects(t *testing.T) {
	t.Parallel()
	svc := New(testSecret)

	now := time.Now().UTC()
	exp := now.Add(time.Minute).Unix()

	for name, claims := range map[string]jwtv5.MapClaims{
		"просрочен":       {"typ": tokenTypeAccess, "id": 42, "ver": 0, "exp": now.Add(-time.Minute).Unix()},
		"без id":          {"typ": tokenTypeAccess, "ver": 0, "exp": exp},
		"ver не число":    {"typ": tokenTypeAccess, "id": 42, "ver": "x", "exp": exp},
		"ver объектом":    {"typ": tokenTypeAccess, "id": 42, "ver": map[string]any{}, "exp": exp},
		"неизвестный typ": {"typ": "other", "id": 42, "ver": 0, "exp": exp},
	} {
		_, _, err := svc.ParseAccessToken(signTestClaims(t, testSecret, claims))
		require.Error(t, err, name)
	}

	// алгоритм none не принимается
	none, err := jwtv5.NewWithClaims(jwtv5.SigningMethodNone, jwtv5.MapClaims{
		"typ": tokenTypeAccess, "id": 42, "ver": 0, "exp": exp,
	}).SignedString(jwtv5.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, _, err = svc.ParseAccessToken(none)
	require.Error(t, err)
}
