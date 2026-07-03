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

	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	apikeyUsc "github.com/rendau/kusec/internal/usecase/apikey"
	appUsc "github.com/rendau/kusec/internal/usecase/app"
	configitemUsc "github.com/rendau/kusec/internal/usecase/configitem"
	configmapUsc "github.com/rendau/kusec/internal/usecase/configmap"
	itemUsc "github.com/rendau/kusec/internal/usecase/item"
	kubeUsc "github.com/rendau/kusec/internal/usecase/kube"
	secretUsc "github.com/rendau/kusec/internal/usecase/secret"
)

const (
	serverVersion = "3.0.0"

	// sessionIdleTimeout — простаивающие MCP-сессии закрываются (вместе с
	// per-session реестром значений).
	sessionIdleTimeout = time.Hour

	// serverInstructions отдаётся клиенту при initialize — правила работы
	// с сервером для AI-агента.
	serverInstructions = `kusec — менеджер секретов и конфигов для Kubernetes. Иерархия: app → secret/configmap → item (ключ-значение).

Правила работы:
1. Запись работает в любом доступном тебе app: create_secret/create_configmap принимают app (id, slug_name или имя), update/create item-ов адресуются по id секрета/конфигмапа. Delete-инструментов нет.
2. Значения секретов тебе недоступны и не нужны: при чтении вместо value отдаются value_chars, value_bytes и усечённый value_sha256 (по нему сравнивай значения между собой). Не запрашивай значения секретов у пользователя и не придумывай их сам.
3. Значения item-ов секрета задаются только декларативно через value_source:
   - generate — сгенерировать случайное значение (format: alnum|ascii|digits|hex|base64url|uuid; length, по умолчанию 32). Укажи name, чтобы значение можно было переиспользовать.
   - reuse — использовать ранее сгенерированное/скопированное значение по name (одно значение в нескольких item-ах, например пароль БД в POSTGRES_PASSWORD и в DATABASE_URL_PASSWORD). Реестр имён живёт в памяти сессии; текущие имена показывает list_value_name.
   - copy_item — скопировать значение существующего item по item_id (из любого доступного app).
   - literal — явное значение, только для несекретного (хосты, порты, url, имена БД).
4. Значения item-ов конфигмапов не секретны: видны полностью и задаются явно.
5. Пагинация zero-based: page начинается с 0, page_size по умолчанию 100.
6. Инструмент sync применяет секреты и конфигмапы в Kubernetes-кластер: укажи app либо all_apps=true. Вызывай его после завершения изменений, а не после каждого item-а; работает только когда kusec запущен внутри кластера.

Типовой сценарий:
create_secret {"app": "billing", "slug_name": "db"} → create_item {"secret_id": "…", "key": "POSTGRES_PASSWORD", "value_source": {"kind": "generate", "name": "db_password"}} → create_item {"secret_id": "…", "key": "DATABASE_URL_PASSWORD", "value_source": {"kind": "reuse", "name": "db_password"}} → sync {"app": "billing"}`
)

type Handler struct {
	sessionSvc *sessionService.Service

	apikeyUsecase     *apikeyUsc.Usecase
	appUsecase        *appUsc.Usecase
	secretUsecase     *secretUsc.Usecase
	itemUsecase       *itemUsc.Usecase
	configmapUsecase  *configmapUsc.Usecase
	configitemUsecase *configitemUsc.Usecase
	kubeUsecase       *kubeUsc.Usecase
}

func New(
	sessionSvc *sessionService.Service,
	apikeyUsecase *apikeyUsc.Usecase,
	appUsecase *appUsc.Usecase,
	secretUsecase *secretUsc.Usecase,
	itemUsecase *itemUsc.Usecase,
	configmapUsecase *configmapUsc.Usecase,
	configitemUsecase *configitemUsc.Usecase,
	kubeUsecase *kubeUsc.Usecase,
) *Handler {
	return &Handler{
		sessionSvc:        sessionSvc,
		apikeyUsecase:     apikeyUsecase,
		appUsecase:        appUsecase,
		secretUsecase:     secretUsecase,
		itemUsecase:       itemUsecase,
		configmapUsecase:  configmapUsecase,
		configitemUsecase: configitemUsecase,
		kubeUsecase:       kubeUsecase,
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
