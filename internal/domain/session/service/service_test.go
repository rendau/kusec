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

	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	refresh, err := svc.CreateRefreshToken(42, passwordHash, 3, start)
	require.NoError(t, err)

	usrId, err := svc.RefreshTokenUserId(refresh)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)

	usrId, ver, sessionStart, err := svc.ParseRefreshToken(refresh, passwordHash)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)
	assert.EqualValues(t, 3, ver)
	assert.True(t, start.Equal(sessionStart), "время входа переносится через токен")

	// смена пароля отзывает refresh-токен
	_, _, _, err = svc.ParseRefreshToken(refresh, "$2a$10$other")
	require.Error(t, err)

	// типы токенов не взаимозаменяемы
	_, _, err = svc.ParseAccessToken(refresh)
	require.Error(t, err)

	access, err := svc.CreateToken(42, 3)
	require.NoError(t, err)
	_, _, _, err = svc.ParseRefreshToken(access, passwordHash)
	require.Error(t, err)
	_, err = svc.RefreshTokenUserId(access)
	require.Error(t, err)
}

// Общий срок сессии считается от входа по паролю и продлением не сдвигается:
// refresh-токен истекает не позже входа + maxSessionAge, а сессию старше
// этого срока не продлить, даже если сам токен ещё не истёк.
func TestRefreshToken_MaxSessionAge(t *testing.T) {
	t.Parallel()
	svc := New(testSecret)

	const passwordHash = "$2a$10$hash"
	now := time.Now().UTC()

	// вход 6 дней назад: токен живёт до входа + maxSessionAge, а не 30 дней
	start := now.Add(-6 * 24 * time.Hour).Truncate(time.Second)
	refresh, err := svc.CreateRefreshToken(42, passwordHash, 0, start)
	require.NoError(t, err)
	claims := jwtv5.MapClaims{}
	_, _, err = jwtv5.NewParser().ParseUnverified(refresh, claims)
	require.NoError(t, err)
	assert.EqualValues(t, start.Add(maxSessionAge).Unix(), claims["exp"])
	assert.EqualValues(t, start.Unix(), claims["ast"])

	_, _, sessionStart, err := svc.ParseRefreshToken(refresh, passwordHash)
	require.NoError(t, err)
	assert.True(t, start.Equal(sessionStart))

	// вход раньше maxSessionAge — новый токен не выдаётся
	_, err = svc.CreateRefreshToken(42, passwordHash, 0, now.Add(-maxSessionAge-time.Second))
	require.Error(t, err)

	// токен ещё не истёк, но сессия старше maxSessionAge — отказ; для старых
	// токенов без ast время входа — момент выдачи (iat)
	old := now.Add(-maxSessionAge - time.Hour).Unix()
	exp := now.Add(time.Hour).Unix()
	fp := passwordFingerprint(passwordHash)
	for name, c := range map[string]jwtv5.MapClaims{
		"ast старше срока":    {"typ": tokenTypeRefresh, "id": 42, "pwd": fp, "ast": old, "iat": now.Unix(), "exp": exp},
		"без ast, iat старый": {"typ": tokenTypeRefresh, "id": 42, "pwd": fp, "iat": old, "exp": exp},
		"без ast и iat":       {"typ": tokenTypeRefresh, "id": 42, "pwd": fp, "exp": exp},
		"ast не число":        {"typ": tokenTypeRefresh, "id": 42, "pwd": fp, "ast": "x", "exp": exp},
	} {
		_, _, _, err = svc.ParseRefreshToken(signTestClaims(t, testSecret, c), passwordHash)
		require.Error(t, err, name)
	}
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
	issued := now.Add(-time.Hour).Truncate(time.Second)
	legacyRefresh := signTestClaims(t, testSecret, jwtv5.MapClaims{
		"typ": tokenTypeRefresh, "id": 42, "pwd": passwordFingerprint(passwordHash), "iat": issued.Unix(), "exp": exp,
	})
	usrId, ver, sessionStart, err := svc.ParseRefreshToken(legacyRefresh, passwordHash)
	require.NoError(t, err)
	assert.EqualValues(t, 42, usrId)
	assert.EqualValues(t, 0, ver)
	assert.True(t, issued.Equal(sessionStart), "без ast время входа — iat")
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
