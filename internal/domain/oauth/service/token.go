package service

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/rendau/kusec/internal/domain/oauth/model"
)

const (
	// AccessTokenPrefix отличает OAuth access-токен от api-ключа (ksk_) на
	// MCP-порту.
	AccessTokenPrefix  = "kso_"
	RefreshTokenPrefix = "ksr_"
	CodePrefix         = "ksc_"

	tokenRandomBytes = 32

	pkceVerifierMinLen = 43
	pkceVerifierMaxLen = 128
)

var tokenPrefixes = map[string]string{
	model.TokenKindCode:    CodePrefix,
	model.TokenKindAccess:  AccessTokenPrefix,
	model.TokenKindRefresh: RefreshTokenPrefix,
}

// GenerateToken возвращает новый токен вида kind и его хэш для хранения.
func GenerateToken(kind string) (token, hash string, err error) {
	raw := make([]byte, tokenRandomBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}

	token = tokenPrefixes[kind] + hex.EncodeToString(raw)

	return token, HashToken(token), nil
}

// HashToken — sha256 токена: токены высокоэнтропийные, соль не нужна.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// VerifyPKCE сверяет code_verifier с code_challenge метода S256 (RFC 7636).
func VerifyPKCE(challenge, verifier string) bool {
	if len(verifier) < pkceVerifierMinLen || len(verifier) > pkceVerifierMaxLen {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}

// RedirectAllowed — адрес возврата совпадает с одним из разрешённых. Адрес без
// пути разрешает любой путь на своём origin; loopback (localhost, 127.0.0.1,
// [::1]) — любой порт: нативные клиенты вроде Claude Code слушают случайный
// порт (RFC 8252, 7.3).
func RedirectAllowed(allowed []string, uri string) bool {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Fragment != "" || u.User != nil {
		return false
	}

	for _, item := range allowed {
		a, err := url.Parse(item)
		if err != nil {
			continue
		}
		if !strings.EqualFold(a.Scheme, u.Scheme) || !strings.EqualFold(a.Hostname(), u.Hostname()) {
			continue
		}
		if !isLoopback(a.Hostname()) && a.Port() != u.Port() {
			continue
		}
		if path := strings.TrimSuffix(a.Path, "/"); path != "" && path != u.Path {
			continue
		}
		return true
	}

	return false
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
