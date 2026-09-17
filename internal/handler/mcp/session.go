package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	appModel "github.com/rendau/kusec/internal/domain/app/model"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/util"
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

	// request id на каждый вызов инструмента — для записей аудита
	ctx = util.RequestIdToContext(ctx, util.NewRequestId())

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
	Kind     string                   `json:"kind" jsonschema:"источник значения: generate (сгенерировать случайное) | reuse (взять ранее сгенерированное по имени) | copy_item (скопировать значение существующего item) | template (собрать значение из шаблона с подстановками) | literal (явное несекретное значение)"`
	Name     string                   `json:"name,omitempty" jsonschema:"имя в реестре значений сессии: для generate/copy_item/template — сохранить под этим именем для последующего reuse, для reuse — какое значение взять"`
	Format   string                   `json:"format,omitempty" jsonschema:"generate: формат значения — alnum (по умолчанию) | ascii | digits | hex | base64url | uuid"`
	Length   int                      `json:"length,omitempty" jsonschema:"generate: длина в символах, по умолчанию 32"`
	ItemId   string                   `json:"item_id,omitempty" jsonschema:"copy_item: id item-а, значение которого скопировать (из любого доступного app)"`
	Value    string                   `json:"value,omitempty" jsonschema:"literal: явное значение (только для несекретных данных: хосты, порты, url и т.п.)"`
	Template string                   `json:"template,omitempty" jsonschema:"template: шаблон значения с плейсхолдерами {{имя}} (например postgres://app:{{db_password}}@pg:5432/app); имя резолвится через vars либо из реестра значений сессии"`
	Vars     map[string]TemplateVarIn `json:"vars,omitempty" jsonschema:"template: источники значений переменных шаблона по имени; переменная без записи в vars берётся из реестра сессии"`
}

// TemplateVarIn — источник значения одной переменной шаблона: те же kind-ы,
// что и у value_source, кроме вложенного template.
type TemplateVarIn struct {
	Kind   string `json:"kind" jsonschema:"источник значения переменной: generate | reuse | copy_item | literal"`
	Name   string `json:"name,omitempty" jsonschema:"имя в реестре значений сессии: для generate/copy_item — сохранить под этим именем, для reuse — какое значение взять"`
	Format string `json:"format,omitempty" jsonschema:"generate: формат значения — alnum (по умолчанию) | ascii | digits | hex | base64url | uuid"`
	Length int    `json:"length,omitempty" jsonschema:"generate: длина в символах, по умолчанию 32"`
	ItemId string `json:"item_id,omitempty" jsonschema:"copy_item: id item-а, значение которого скопировать (из любого доступного app)"`
	Value  string `json:"value,omitempty" jsonschema:"literal: явное значение (только для несекретных данных)"`
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

	case "template":
		return s.resolveTemplate(ctx, src)

	case "literal":
		return src.Value, nil

	default:
		return "", fmt.Errorf("неизвестный kind %q (доступны: generate, reuse, copy_item, template, literal)", src.Kind)
	}
}

// templatePlaceholderRe — плейсхолдер {{имя}} в шаблоне значения.
var templatePlaceholderRe = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_][A-Za-z0-9_.-]*)\s*\}\}`)

// resolveTemplate собирает значение из шаблона: каждый плейсхолдер {{имя}}
// резолвится через vars либо из реестра значений сессии. Итог агенту не
// раскрывается (помечается для скраба ошибок).
func (s *sessionServer) resolveTemplate(ctx context.Context, src ValueSourceIn) (string, error) {
	if src.Template == "" {
		return "", errors.New("template: требуется template")
	}

	matches := templatePlaceholderRe.FindAllStringSubmatchIndex(src.Template, -1)
	if len(matches) == 0 {
		return "", errors.New("template: в шаблоне нет ни одного плейсхолдера {{имя}}; для значения без переменных используй literal")
	}

	resolved := map[string]string{}

	var b strings.Builder
	prev := 0
	for _, m := range matches {
		b.WriteString(src.Template[prev:m[0]])

		name := src.Template[m[2]:m[3]]
		value, ok := resolved[name]
		if !ok {
			var err error
			if value, err = s.resolveTemplateVar(ctx, name, src.Vars); err != nil {
				return "", err
			}
			resolved[name] = value
		}

		b.WriteString(value)
		prev = m[1]
	}
	b.WriteString(src.Template[prev:])

	value := b.String()
	s.vault.markSeen(value)
	if src.Name != "" {
		s.vault.remember(src.Name, value)
	}

	return value, nil
}

// resolveTemplateVar — значение одной переменной шаблона: явное описание в
// vars либо реестр значений сессии по имени.
func (s *sessionServer) resolveTemplateVar(ctx context.Context, name string, vars map[string]TemplateVarIn) (string, error) {
	if v, ok := vars[name]; ok {
		value, err := s.resolveValueSource(ctx, ValueSourceIn{
			Kind:   v.Kind,
			Name:   v.Name,
			Format: v.Format,
			Length: v.Length,
			ItemId: v.ItemId,
			Value:  v.Value,
		})
		if err != nil {
			return "", fmt.Errorf("template: переменная %q: %w", name, err)
		}
		return value, nil
	}

	if value, ok := s.vault.lookup(name); ok {
		return value, nil
	}

	return "", fmt.Errorf("template: переменная %q не описана в vars и не найдена в реестре сессии (доступные имена: [%s])", name, strings.Join(s.vault.names(), ", "))
}
