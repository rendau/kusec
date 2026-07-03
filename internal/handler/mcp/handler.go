// Package mcp — встроенный MCP-сервер (Model Context Protocol) поверх
// usecase-слоя kusec для AI-агентов. Streamable HTTP transport на отдельном
// порту (config MCP_ENABLED / MCP_PORT), аутентификация по api-key.
//
// Модель безопасности: агент видит всё, кроме значений секретов. Значения
// item-ов маскируются (длина + усечённый hash), новые значения агент задаёт
// декларативно через value_source (generate / reuse / copy_item / literal) —
// сами значения в ответы инструментов не попадают. Ключи с mcp_only=true
// работают только здесь, основной API их отвергает.
package mcp

import (
	"context"
	"net/http"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	sessionService "github.com/mechta-market/kusec/internal/domain/session/service"
	apikeyUsc "github.com/mechta-market/kusec/internal/usecase/apikey"
	appUsc "github.com/mechta-market/kusec/internal/usecase/app"
	configitemUsc "github.com/mechta-market/kusec/internal/usecase/configitem"
	configmapUsc "github.com/mechta-market/kusec/internal/usecase/configmap"
	itemUsc "github.com/mechta-market/kusec/internal/usecase/item"
	secretUsc "github.com/mechta-market/kusec/internal/usecase/secret"
)

const (
	serverVersion = "2.0.0"

	// sessionIdleTimeout — простаивающие MCP-сессии закрываются (вместе с
	// per-session реестром значений).
	sessionIdleTimeout = time.Hour
)

type Handler struct {
	sessionSvc *sessionService.Service

	apikeyUsecase     *apikeyUsc.Usecase
	appUsecase        *appUsc.Usecase
	secretUsecase     *secretUsc.Usecase
	itemUsecase       *itemUsc.Usecase
	configmapUsecase  *configmapUsc.Usecase
	configitemUsecase *configitemUsc.Usecase
}

func New(
	sessionSvc *sessionService.Service,
	apikeyUsecase *apikeyUsc.Usecase,
	appUsecase *appUsc.Usecase,
	secretUsecase *secretUsc.Usecase,
	itemUsecase *itemUsc.Usecase,
	configmapUsecase *configmapUsc.Usecase,
	configitemUsecase *configitemUsc.Usecase,
) *Handler {
	return &Handler{
		sessionSvc:        sessionSvc,
		apikeyUsecase:     apikeyUsecase,
		appUsecase:        appUsecase,
		secretUsecase:     secretUsecase,
		itemUsecase:       itemUsecase,
		configmapUsecase:  configmapUsecase,
		configitemUsecase: configitemUsecase,
	}
}

type ctxKeyT int

const (
	ctxKeySession ctxKeyT = iota
	ctxKeyHash
)

// HTTPHandler — streamable HTTP endpoint MCP c аутентификацией по api-key.
func (h *Handler) HTTPHandler() http.Handler {
	streamable := mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		session, _ := r.Context().Value(ctxKeySession).(*sessionModel.Session)
		keyHash, _ := r.Context().Value(ctxKeyHash).(string)
		if session == nil || keyHash == "" {
			return nil
		}
		return h.newSessionServer(session, keyHash)
	}, &mcpsdk.StreamableHTTPOptions{
		SessionTimeout: sessionIdleTimeout,
	})

	return h.authMiddleware(streamable)
}

// authMiddleware проверяет api-ключ на каждом HTTP-запросе.
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := bearerToken(r.Header.Get("Authorization"))
		if key == "" {
			http.Error(w, "kusec mcp: требуется api-ключ (Authorization: Bearer ksk_...)", http.StatusUnauthorized)
			return
		}

		session, err := h.apikeyUsecase.McpSessionFromKey(r.Context(), key)
		if err != nil {
			http.Error(w, "kusec mcp: недействительный api-ключ", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeySession, session)
		ctx = context.WithValue(ctx, ctxKeyHash, hashKey(key))

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(headerValue string) string {
	value := strings.TrimSpace(headerValue)
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}

	return ""
}
