package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/errs"
)

const (
	// accessTokenTTL — короткий срок жизни access-токена: окно утечки
	// ограничено, продление идёт через refresh-токен.
	accessTokenTTL = 15 * time.Minute
	// refreshTokenTTL — срок жизни refresh-токена (максимальная длина сессии
	// без повторного логина).
	refreshTokenTTL = 30 * 24 * time.Hour

	// Значения claim "typ", разделяющие назначение токенов. Access-токены,
	// выданные до введения refresh-flow, не имеют "typ" — они принимаются
	// как access до их естественного истечения.
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

type contextKey string

const sessionContextKey = contextKey("session")

type Service struct {
	secret string
}

func New(secret string) *Service {
	return &Service{
		secret: strings.TrimSpace(secret),
	}
}

// WithContext кладёт сессию в контекст (вызывается из gRPC-интерцептора).
func (s *Service) WithContext(ctx context.Context, session *sessionModel.Session) context.Context {
	if session == nil {
		session = sessionModel.New(0)
	}
	return context.WithValue(ctx, sessionContextKey, session)
}

// FromContext извлекает сессию из контекста; всегда возвращает не-nil.
func (s *Service) FromContext(ctx context.Context) *sessionModel.Session {
	if ctx != nil {
		if session, ok := ctx.Value(sessionContextKey).(*sessionModel.Session); ok && session != nil {
			return session
		}
	}
	return sessionModel.New(0)
}

func (s *Service) CtxIsAuthorized(ctx context.Context) bool {
	return s.FromContext(ctx).IsAuthorized()
}

func (s *Service) CtxIsAdmin(ctx context.Context) bool {
	return s.FromContext(ctx).IsAdmin()
}

// parseClaims парсит и валидирует подпись/срок HS256-токена.
func (s *Service) parseClaims(tokenStr string) (jwtv5.MapClaims, error) {
	if s.secret == "" {
		return nil, errs.InvalidConfig
	}

	claims := jwtv5.MapClaims{}
	parsedToken, err := jwtv5.ParseWithClaims(
		tokenStr,
		claims,
		func(_ *jwtv5.Token) (any, error) {
			return []byte(s.secret), nil
		},
		jwtv5.WithValidMethods([]string{jwtv5.SigningMethodHS256.Alg()}),
	)
	if err != nil || parsedToken == nil || !parsedToken.Valid {
		if err == nil {
			return nil, fmt.Errorf("fail to parse token")
		}
		return nil, err
	}

	return claims, nil
}

func tokenType(claims jwtv5.MapClaims) string {
	if raw, ok := claims["typ"].(string); ok {
		return raw
	}
	return ""
}

// ParseAccessToken парсит и валидирует access-токен и возвращает id
// пользователя и номер поколения сессий (claim "ver"). Прав в токене нет:
// сессию по живой записи пользователя строит вызывающий (usr usecase), он же
// сверяет ver с usr.session_version.
// Refresh-токены здесь отвергаются: они годятся только для RefreshToken.
func (s *Service) ParseAccessToken(tokenStr string) (usrId, ver int64, err error) {
	claims, err := s.parseClaims(tokenStr)
	if err != nil {
		return 0, 0, err
	}

	// Токены без "typ" выданы до введения refresh-flow — принимаем как access.
	if typ := tokenType(claims); typ != "" && typ != tokenTypeAccess {
		return 0, 0, fmt.Errorf("not an access token")
	}

	usrRaw, ok := claims["id"]
	if !ok {
		return 0, 0, fmt.Errorf("missing user id claim in token")
	}
	usrId, err = usrIDFromClaim(usrRaw)
	if err != nil {
		return 0, 0, err
	}

	ver, err = versionFromClaims(claims)
	if err != nil {
		return 0, 0, err
	}

	return usrId, ver, nil
}

// CreateToken подписывает короткоживущий access-токен: id пользователя и
// номер поколения его сессий (usr.session_version). Права в токен не
// зашиваются — они берутся из записи пользователя на каждый запрос.
func (s *Service) CreateToken(usrId, ver int64) (string, error) {
	now := time.Now().UTC()
	return s.signClaims(jwtv5.MapClaims{
		"typ": tokenTypeAccess,
		"id":  usrId,
		"ver": ver,
		"iat": now.Unix(),
		"exp": now.Add(accessTokenTTL).Unix(),
	})
}

// passwordFingerprint — необратимый отпечаток хеша пароля, зашиваемый в
// refresh-токен: смена пароля инвалидирует все ранее выданные refresh-токены.
func passwordFingerprint(passwordHash string) string {
	sum := sha256.Sum256([]byte(passwordHash))
	return hex.EncodeToString(sum[:8])
}

// CreateRefreshToken подписывает долгоживущий refresh-токен.
func (s *Service) CreateRefreshToken(usrId int64, passwordHash string, ver int64) (string, error) {
	now := time.Now().UTC()
	return s.signClaims(jwtv5.MapClaims{
		"typ": tokenTypeRefresh,
		"id":  usrId,
		"ver": ver,
		"pwd": passwordFingerprint(passwordHash),
		"iat": now.Unix(),
		"exp": now.Add(refreshTokenTTL).Unix(),
	})
}

// ParseRefreshToken валидирует refresh-токен и возвращает id пользователя и
// номер поколения сессий (claim "ver" — вызывающий сверяет его с
// usr.session_version). Отпечаток пароля сверяется с актуальным хешем —
// после смены пароля токен считается отозванным.
func (s *Service) ParseRefreshToken(tokenStr, currentPasswordHash string) (usrId, ver int64, err error) {
	claims, err := s.parseClaims(tokenStr)
	if err != nil {
		return 0, 0, err
	}

	if tokenType(claims) != tokenTypeRefresh {
		return 0, 0, fmt.Errorf("not a refresh token")
	}

	fp, _ := claims["pwd"].(string)
	if fp == "" || fp != passwordFingerprint(currentPasswordHash) {
		return 0, 0, fmt.Errorf("refresh token revoked by password change")
	}

	usrRaw, ok := claims["id"]
	if !ok {
		return 0, 0, fmt.Errorf("missing user id claim in token")
	}
	usrId, err = usrIDFromClaim(usrRaw)
	if err != nil {
		return 0, 0, err
	}

	ver, err = versionFromClaims(claims)
	if err != nil {
		return 0, 0, err
	}

	return usrId, ver, nil
}

// versionFromClaims — claim "ver". Токены без него выданы до введения
// session_version и считаются поколением 0: они работают, пока пользователь
// не сменил пароль и не вышел.
func versionFromClaims(claims jwtv5.MapClaims) (int64, error) {
	raw, ok := claims["ver"]
	if !ok {
		return 0, nil
	}
	ver, err := usrIDFromClaim(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid ver claim in token")
	}
	return ver, nil
}

// RefreshTokenUserId извлекает id пользователя из refresh-токена (после
// проверки подписи/срока/типа), не сверяя отпечаток пароля — нужен, чтобы
// сначала загрузить пользователя и его актуальный хеш.
func (s *Service) RefreshTokenUserId(tokenStr string) (int64, error) {
	claims, err := s.parseClaims(tokenStr)
	if err != nil {
		return 0, err
	}
	if tokenType(claims) != tokenTypeRefresh {
		return 0, fmt.Errorf("not a refresh token")
	}
	usrRaw, ok := claims["id"]
	if !ok {
		return 0, fmt.Errorf("missing user id claim in token")
	}
	return usrIDFromClaim(usrRaw)
}

func (s *Service) signClaims(claims jwtv5.MapClaims) (string, error) {
	if s.secret == "" {
		return "", errs.InvalidConfig
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", fmt.Errorf("token.SignedString: %w", err)
	}

	return tokenStr, nil
}

func usrIDFromClaim(v any) (int64, error) {
	switch x := v.(type) {
	case float64:
		return int64(x), nil
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case string:
		parsedId, err := strconv.ParseInt(x, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("strconv.ParseInt: %w", err)
		}
		return parsedId, nil
	default:
		return 0, fmt.Errorf("invalid user id claim")
	}
}
