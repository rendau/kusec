package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	appModel "github.com/rendau/kusec/internal/domain/app/model"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
)

func hashKey(key string) string {
	return apikeyService.HashKey(key)
}

// sessionServer — состояние одной MCP-сессии: пользователь (по api-ключу)
// и реестр значений. Живёт в памяти до закрытия сессии.
type sessionServer struct {
	h       *Handler
	keyHash string
	vault   *vault
}

func (h *Handler) newSessionServer(_ *sessionModel.Session, keyHash string) *mcpsdk.Server {
	s := &sessionServer{
		h:       h,
		keyHash: keyHash,
		vault:   newVault(),
	}

	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "kusec",
		Title:   "Kusec — менеджер секретов и конфигов",
		Version: serverVersion,
	}, &mcpsdk.ServerOptions{
		Instructions: serverInstructions,
	})

	s.registerReadTools(srv)
	s.registerWriteTools(srv)
	s.registerKubeTools(srv)

	return srv
}

// toolCtx — вход каждого инструмента: сверяет ключ запроса с ключом сессии
// (защита от подмены Mcp-Session-Id) и кладёт свежую сессию пользователя в
// контекст для проверок usecase-слоя.
func (s *sessionServer) toolCtx(ctx context.Context, req *mcpsdk.CallToolRequest) (context.Context, error) {
	key := ""
	if req != nil && req.Extra != nil && req.Extra.Header != nil {
		key = bearerToken(req.Extra.Header.Get("Authorization"))
	}
	if key == "" || hashKey(key) != s.keyHash {
		return nil, errors.New("api-ключ запроса не совпадает с ключом MCP-сессии")
	}

	session, err := s.h.apikeyUsecase.McpSessionFromKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("api-ключ недействителен: %w", err)
	}

	return s.h.sessionSvc.WithContext(ctx, session), nil
}

// toolErr готовит ошибку к выдаче агенту: вычищает известные значения секретов.
func (s *sessionServer) toolErr(err error) error {
	return errors.New(s.vault.scrub(err.Error()))
}

// resolveApp находит app по id, slug_name или точному имени.
func (s *sessionServer) resolveApp(ctx context.Context, ref string) (*appModel.Main, error) {
	app, getErr := s.h.appUsecase.Get(ctx, ref)
	if getErr == nil {
		return app, nil
	}

	candidates, _, err := s.h.appUsecase.List(ctx, &appModel.ListReq{
		ListParams: commonModel.ListParams{PageSize: 100},
		Search:     &ref,
	})
	if err != nil {
		return nil, fmt.Errorf("поиск app %q: %w", ref, err)
	}

	exact := lo.Filter(candidates, func(a *appModel.Main, _ int) bool {
		return a.SlugName == ref || a.Name == ref
	})
	if len(exact) > 0 {
		candidates = exact
	}

	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("app %q не найден: %w", ref, getErr)
	case 1:
		return candidates[0], nil
	default:
		slugs := lo.Map(candidates, func(a *appModel.Main, _ int) string { return a.SlugName })
		return nil, fmt.Errorf("app %q неоднозначен, кандидаты: %s", ref, strings.Join(slugs, ", "))
	}
}

// ── value_source ────────────────────────────────────────

// ValueSourceIn — декларативный источник значения item-а: агент описывает,
// откуда взять значение, но само значение никогда не видит.
type ValueSourceIn struct {
	Kind   string `json:"kind" jsonschema:"источник значения: generate (сгенерировать случайное) | reuse (взять ранее сгенерированное по имени) | copy_item (скопировать значение существующего item) | literal (явное несекретное значение)"`
	Name   string `json:"name,omitempty" jsonschema:"имя в реестре значений сессии: для generate/copy_item — сохранить под этим именем для последующего reuse, для reuse — какое значение взять"`
	Format string `json:"format,omitempty" jsonschema:"generate: формат значения — alnum (по умолчанию) | ascii | digits | hex | base64url | uuid"`
	Length int    `json:"length,omitempty" jsonschema:"generate: длина в символах, по умолчанию 32"`
	ItemId string `json:"item_id,omitempty" jsonschema:"copy_item: id item-а, значение которого скопировать (из любого доступного app)"`
	Value  string `json:"value,omitempty" jsonschema:"literal: явное значение (только для несекретных данных: хосты, порты, url и т.п.)"`
}

// resolveValueSource возвращает готовое значение для записи.
func (s *sessionServer) resolveValueSource(ctx context.Context, src ValueSourceIn) (string, error) {
	switch src.Kind {
	case "generate":
		value, err := generateValue(src.Format, src.Length)
		if err != nil {
			return "", fmt.Errorf("generate: %w", err)
		}
		s.vault.markSeen(value)
		if src.Name != "" {
			s.vault.remember(src.Name, value)
		}
		return value, nil

	case "reuse":
		if src.Name == "" {
			return "", errors.New("reuse: требуется name")
		}
		value, ok := s.vault.lookup(src.Name)
		if !ok {
			return "", fmt.Errorf("reuse: значение %q не найдено в реестре сессии (реестр живёт в памяти сессии; доступные имена: [%s]); для существующих значений используй copy_item", src.Name, strings.Join(s.vault.names(), ", "))
		}
		return value, nil

	case "copy_item":
		if src.ItemId == "" {
			return "", errors.New("copy_item: требуется item_id")
		}
		item, err := s.h.itemUsecase.Get(ctx, src.ItemId)
		if err != nil {
			return "", fmt.Errorf("copy_item: %w", err)
		}
		s.vault.markSeen(item.Value)
		if src.Name != "" {
			s.vault.remember(src.Name, item.Value)
		}
		return item.Value, nil

	case "literal":
		return src.Value, nil

	default:
		return "", fmt.Errorf("неизвестный kind %q (доступны: generate, reuse, copy_item, literal)", src.Kind)
	}
}
