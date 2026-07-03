// Package mcpserver — MCP-сервер поверх API kusec для AI-агентов.
//
// Модель безопасности: агент видит всё, кроме значений секретов. Значения
// item-ов маскируются (длина + усечённый hash), новые значения агент задаёт
// декларативно через value_source (generate / reuse / copy_item / literal),
// а сами значения существуют только внутри процесса MCP-сервера.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	"github.com/mechta-market/kusec/internal/mcpserver/client"
	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

const serverVersion = "1.0.0"

type Server struct {
	api   *client.Client
	vault *vault

	mu     sync.Mutex
	curApp *model.App
}

func New(cfg Config) *Server {
	return &Server{
		api:   client.New(cfg.ApiURL, cfg.ApiKey, cfg.Username, cfg.Password, cfg.RefreshToken, cfg.InsecureSkipVerify),
		vault: newVault(),
	}
}

func (s *Server) Run(ctx context.Context) error {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "kusec",
		Title:   "Kusec — менеджер секретов и конфигов",
		Version: serverVersion,
	}, nil)

	s.registerReadTools(srv)
	s.registerWriteTools(srv)

	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server run: %w", err)
	}

	return nil
}

// toolErr готовит ошибку к выдаче агенту: вычищает известные значения секретов.
func (s *Server) toolErr(err error) error {
	return errors.New(s.vault.scrub(err.Error()))
}

// ── Текущий app ─────────────────────────────────────────

func (s *Server) currentApp() (model.App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.curApp == nil {
		return model.App{}, errors.New("текущий app не выбран: сначала вызови use_app (или create_app)")
	}

	return *s.curApp, nil
}

func (s *Server) setCurrentApp(app model.App) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.curApp = &app
}

// resolveApp находит app по id, slug_name или точному имени.
func (s *Server) resolveApp(ctx context.Context, ref string) (model.App, error) {
	app, getErr := s.api.AppGet(ctx, ref)
	if getErr == nil {
		return app, nil
	}

	rep, err := s.api.AppList(ctx, model.AppListReq{
		ListParams: model.ListParams{PageSize: 100},
		Search:     &ref,
	})
	if err != nil {
		return model.App{}, fmt.Errorf("поиск app %q: %w", ref, err)
	}

	candidates := rep.Results
	exact := lo.Filter(candidates, func(a model.App, _ int) bool {
		return a.SlugName == ref || a.Name == ref
	})
	if len(exact) > 0 {
		candidates = exact
	}

	switch len(candidates) {
	case 0:
		return model.App{}, fmt.Errorf("app %q не найден: %w", ref, getErr)
	case 1:
		return candidates[0], nil
	default:
		slugs := lo.Map(candidates, func(a model.App, _ int) string { return a.SlugName })
		return model.App{}, fmt.Errorf("app %q неоднозначен, кандидаты: %s", ref, strings.Join(slugs, ", "))
	}
}

// ── Scope-проверки: запись только в текущем app ─────────

func (s *Server) secretInCurrentApp(ctx context.Context, secretID string) (model.Secret, error) {
	app, err := s.currentApp()
	if err != nil {
		return model.Secret{}, err
	}

	sec, err := s.api.SecretGet(ctx, secretID)
	if err != nil {
		return model.Secret{}, err
	}

	if sec.AppID != app.ID {
		return model.Secret{}, fmt.Errorf("секрет %s (%s) принадлежит другому app (%s): запись разрешена только в текущем app %s — сначала переключись через use_app", sec.ID, sec.SlugName, sec.AppID, app.SlugName)
	}

	return sec, nil
}

func (s *Server) configMapInCurrentApp(ctx context.Context, configmapID string) (model.ConfigMap, error) {
	app, err := s.currentApp()
	if err != nil {
		return model.ConfigMap{}, err
	}

	cm, err := s.api.ConfigMapGet(ctx, configmapID)
	if err != nil {
		return model.ConfigMap{}, err
	}

	if cm.AppID != app.ID {
		return model.ConfigMap{}, fmt.Errorf("configmap %s (%s) принадлежит другому app (%s): запись разрешена только в текущем app %s — сначала переключись через use_app", cm.ID, cm.SlugName, cm.AppID, app.SlugName)
	}

	return cm, nil
}

// itemInCurrentApp возвращает item, если его секрет принадлежит текущему app.
// Значение item-а помечается увиденным для последующего скраба.
func (s *Server) itemInCurrentApp(ctx context.Context, itemID string) (model.Item, error) {
	item, err := s.api.ItemGet(ctx, itemID)
	if err != nil {
		return model.Item{}, err
	}
	s.vault.markSeen(item.Value)

	if _, err = s.secretInCurrentApp(ctx, item.SecretID); err != nil {
		return model.Item{}, err
	}

	return item, nil
}

// ── value_source ────────────────────────────────────────

// ValueSourceIn — декларативный источник значения item-а: агент описывает,
// откуда взять значение, но само значение никогда не видит.
type ValueSourceIn struct {
	Kind   string `json:"kind" jsonschema:"источник значения: generate (сгенерировать случайное) | reuse (взять ранее сгенерированное по имени) | copy_item (скопировать значение существующего item) | literal (явное несекретное значение)"`
	Name   string `json:"name,omitempty" jsonschema:"имя в реестре значений сессии: для generate/copy_item — сохранить под этим именем для последующего reuse, для reuse — какое значение взять"`
	Format string `json:"format,omitempty" jsonschema:"generate: формат значения — alnum (по умолчанию) | ascii | digits | hex | base64url | uuid"`
	Length int    `json:"length,omitempty" jsonschema:"generate: длина в символах, по умолчанию 32"`
	ItemID string `json:"item_id,omitempty" jsonschema:"copy_item: id item-а, значение которого скопировать (можно из любого app)"`
	Value  string `json:"value,omitempty" jsonschema:"literal: явное значение (только для несекретных данных: хосты, порты, url и т.п.)"`
}

// resolveValueSource возвращает готовое значение для записи в kusec.
func (s *Server) resolveValueSource(ctx context.Context, appID string, src ValueSourceIn) (string, error) {
	switch src.Kind {
	case "generate":
		value, err := generateValue(src.Format, src.Length)
		if err != nil {
			return "", fmt.Errorf("generate: %w", err)
		}
		s.vault.markSeen(value)
		if src.Name != "" {
			s.vault.remember(appID, src.Name, value)
		}
		return value, nil

	case "reuse":
		if src.Name == "" {
			return "", errors.New("reuse: требуется name")
		}
		value, ok := s.vault.lookup(appID, src.Name)
		if !ok {
			return "", fmt.Errorf("reuse: значение %q не найдено в реестре текущего app (реестр живёт в памяти сессии; доступные имена: [%s]); для существующих значений используй copy_item", src.Name, strings.Join(s.vault.names(appID), ", "))
		}
		return value, nil

	case "copy_item":
		if src.ItemID == "" {
			return "", errors.New("copy_item: требуется item_id")
		}
		item, err := s.api.ItemGet(ctx, src.ItemID)
		if err != nil {
			return "", fmt.Errorf("copy_item: %w", err)
		}
		s.vault.markSeen(item.Value)
		if src.Name != "" {
			s.vault.remember(appID, src.Name, item.Value)
		}
		return item.Value, nil

	case "literal":
		return src.Value, nil

	default:
		return "", fmt.Errorf("неизвестный kind %q (доступны: generate, reuse, copy_item, literal)", src.Kind)
	}
}
