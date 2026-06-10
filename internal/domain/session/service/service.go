package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	"github.com/mechta-market/kusec/internal/errs"
)

const tokenTTL = 12 * time.Hour

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

// FromToken парсит и валидирует HS256-токен, возвращая сессию.
func (s *Service) FromToken(tokenStr string) (*sessionModel.Session, error) {
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

	usrRaw, ok := claims["id"]
	if !ok {
		return nil, fmt.Errorf("missing user id claim in token")
	}
	usrId, err := usrIDFromClaim(usrRaw)
	if err != nil {
		return nil, err
	}

	isAdminRaw, ok := claims["is_admin"]
	if !ok {
		return nil, fmt.Errorf("missing is_admin claim in token")
	}
	isAdmin, err := boolFromClaim(isAdminRaw)
	if err != nil {
		return nil, err
	}

	return &sessionModel.Session{
		Id:    usrId,
		Admin: isAdmin,
	}, nil
}

// CreateToken подписывает HS256-токен с данными пользователя.
func (s *Service) CreateToken(usrId int64, isAdmin bool) (string, error) {
	if s.secret == "" {
		return "", errs.InvalidConfig
	}

	now := time.Now().UTC()
	claims := jwtv5.MapClaims{
		"id":       usrId,
		"is_admin": isAdmin,
		"iat":      now.Unix(),
		"exp":      now.Add(tokenTTL).Unix(),
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", fmt.Errorf("token.SignedString: %w", err)
	}

	return tokenStr, nil
}

func boolFromClaim(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		parsed, err := strconv.ParseBool(x)
		if err != nil {
			return false, fmt.Errorf("strconv.ParseBool: %w", err)
		}
		return parsed, nil
	case float64:
		switch x {
		case 1:
			return true, nil
		case 0:
			return false, nil
		default:
			return false, fmt.Errorf("invalid bool claim")
		}
	default:
		return false, fmt.Errorf("invalid bool claim")
	}
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
