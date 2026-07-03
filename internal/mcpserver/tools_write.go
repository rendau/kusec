package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

// ── Регистрация ─────────────────────────────────────────

func (s *Server) registerWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "use_app",
		Description: "Выбрать текущий app (по id, slug_name или имени). Все операции записи (create/update секретов, конфигмапов и item-ов) разрешены только в текущем app.",
	}, s.useApp)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "current_app",
		Description: "Показать текущий app и имена значений, зарегистрированных в реестре сессии (сами значения не раскрываются).",
	}, s.currentAppTool)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_app",
		Description: "Создать app. Новый app автоматически становится текущим.",
	}, s.createApp)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_app",
		Description: "Обновить текущий app (только переданные поля).",
	}, s.updateApp)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_secret",
		Description: "Создать секрет в текущем app.",
	}, s.createSecret)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_secret",
		Description: "Обновить секрет текущего app (только переданные поля).",
	}, s.updateSecret)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_configmap",
		Description: "Создать configmap в текущем app.",
	}, s.createConfigMap)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_configmap",
		Description: "Обновить configmap текущего app (только переданные поля).",
	}, s.updateConfigMap)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_item",
		Description: "Создать item секрета в текущем app. Значение задаётся через value_source: generate — сгенерировать случайное (с сохранением в реестре сессии под name), reuse — переиспользовать сгенерированное ранее, copy_item — скопировать значение существующего item (в т.ч. из другого app), literal — явное несекретное значение. Сгенерированные/скопированные значения агенту не показываются.",
	}, s.createItem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_item",
		Description: "Обновить item секрета текущего app (только переданные поля). Значение — через value_source, как в create_item.",
	}, s.updateItem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_config_item",
		Description: "Создать item конфигмапа в текущем app (значение несекретное, задаётся явно).",
	}, s.createConfigItem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_config_item",
		Description: "Обновить item конфигмапа текущего app (только переданные поля).",
	}, s.updateConfigItem)
}

// ── App ─────────────────────────────────────────────────

type UseAppIn struct {
	App string `json:"app" jsonschema:"id, slug_name или имя app"`
}

func (s *Server) useApp(ctx context.Context, _ *mcp.CallToolRequest, in UseAppIn) (*mcp.CallToolResult, AppOut, error) {
	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, AppOut{}, s.toolErr(err)
	}

	s.setCurrentApp(app)

	return nil, appToOut(app, 0), nil
}

type CurrentAppOut struct {
	App        *AppOut  `json:"app,omitempty"`
	ValueNames []string `json:"value_names" jsonschema:"имена значений в реестре сессии для reuse"`
}

func (s *Server) currentAppTool(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, CurrentAppOut, error) {
	app, err := s.currentApp()
	if err != nil {
		return nil, CurrentAppOut{ValueNames: []string{}}, nil
	}

	out := appToOut(app, 0)

	return nil, CurrentAppOut{App: &out, ValueNames: s.vault.names(app.ID)}, nil
}

type CreateAppIn struct {
	Namespace   string `json:"namespace" jsonschema:"k8s namespace приложения"`
	Name        string `json:"name" jsonschema:"человекочитаемое имя"`
	SlugName    string `json:"slug_name" jsonschema:"слаг (входит в имена k8s-объектов)"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type CreateOut struct {
	ID string `json:"id"`
}

func (s *Server) createApp(ctx context.Context, _ *mcp.CallToolRequest, in CreateAppIn) (*mcp.CallToolResult, CreateOut, error) {
	id, err := s.api.AppCreate(ctx, model.AppCreateReq{
		Active:      in.Active,
		Namespace:   in.Namespace,
		Name:        in.Name,
		SlugName:    in.SlugName,
		Description: in.Description,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	app, err := s.api.AppGet(ctx, id)
	if err == nil {
		s.setCurrentApp(app)
	}

	return nil, CreateOut{ID: id}, nil
}

type UpdateAppIn struct {
	Active      *bool   `json:"active,omitempty"`
	Namespace   *string `json:"namespace,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type StatusOut struct {
	OK bool `json:"ok"`
}

func (s *Server) updateApp(ctx context.Context, _ *mcp.CallToolRequest, in UpdateAppIn) (*mcp.CallToolResult, StatusOut, error) {
	app, err := s.currentApp()
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	err = s.api.AppUpdate(ctx, app.ID, model.AppUpdateReq{
		Active:      in.Active,
		Namespace:   in.Namespace,
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	// обновляем локальную копию текущего app
	if fresh, ferr := s.api.AppGet(ctx, app.ID); ferr == nil {
		s.setCurrentApp(fresh)
	}

	return nil, StatusOut{OK: true}, nil
}

// ── Secret ──────────────────────────────────────────────

type CreateSecretIn struct {
	SlugName    string `json:"slug_name" jsonschema:"слаг секрета (входит в имя k8s-секрета)"`
	Description string `json:"description,omitempty"`
	KubeType    string `json:"kube_type,omitempty" jsonschema:"тип k8s-секрета, пусто = Opaque (например kubernetes.io/basic-auth)"`
	Active      *bool  `json:"active,omitempty"`
}

func (s *Server) createSecret(ctx context.Context, _ *mcp.CallToolRequest, in CreateSecretIn) (*mcp.CallToolResult, CreateOut, error) {
	app, err := s.currentApp()
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	id, err := s.api.SecretCreate(ctx, model.SecretCreateReq{
		AppID:       app.ID,
		Active:      in.Active,
		SlugName:    in.SlugName,
		Description: in.Description,
		KubeType:    in.KubeType,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	return nil, CreateOut{ID: id}, nil
}

type UpdateSecretIn struct {
	ID          string  `json:"id" jsonschema:"id секрета"`
	Active      *bool   `json:"active,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
	KubeType    *string `json:"kube_type,omitempty"`
}

func (s *Server) updateSecret(ctx context.Context, _ *mcp.CallToolRequest, in UpdateSecretIn) (*mcp.CallToolResult, StatusOut, error) {
	if _, err := s.secretInCurrentApp(ctx, in.ID); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	err := s.api.SecretUpdate(ctx, in.ID, model.SecretUpdateReq{
		Active:      in.Active,
		SlugName:    in.SlugName,
		Description: in.Description,
		KubeType:    in.KubeType,
	})
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	return nil, StatusOut{OK: true}, nil
}

// ── ConfigMap ───────────────────────────────────────────

type CreateConfigMapIn struct {
	SlugName    string `json:"slug_name" jsonschema:"слаг конфигмапа (входит в имя k8s-configmap)"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

func (s *Server) createConfigMap(ctx context.Context, _ *mcp.CallToolRequest, in CreateConfigMapIn) (*mcp.CallToolResult, CreateOut, error) {
	app, err := s.currentApp()
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	id, err := s.api.ConfigMapCreate(ctx, model.ConfigMapCreateReq{
		AppID:       app.ID,
		Active:      in.Active,
		SlugName:    in.SlugName,
		Description: in.Description,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	return nil, CreateOut{ID: id}, nil
}

type UpdateConfigMapIn struct {
	ID          string  `json:"id" jsonschema:"id конфигмапа"`
	Active      *bool   `json:"active,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (s *Server) updateConfigMap(ctx context.Context, _ *mcp.CallToolRequest, in UpdateConfigMapIn) (*mcp.CallToolResult, StatusOut, error) {
	if _, err := s.configMapInCurrentApp(ctx, in.ID); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	err := s.api.ConfigMapUpdate(ctx, in.ID, model.ConfigMapUpdateReq{
		Active:      in.Active,
		SlugName:    in.SlugName,
		Description: in.Description,
	})
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	return nil, StatusOut{OK: true}, nil
}

// ── Item ────────────────────────────────────────────────

type CreateItemIn struct {
	SecretID    string        `json:"secret_id" jsonschema:"id секрета текущего app"`
	Key         string        `json:"key" jsonschema:"ключ item-а (имя переменной в k8s-секрете)"`
	ValueSource ValueSourceIn `json:"value_source" jsonschema:"источник значения (само значение агенту не раскрывается)"`
	Active      *bool         `json:"active,omitempty"`
	ValueFormat string        `json:"value_format,omitempty"`
	Encoding    string        `json:"encoding,omitempty"`
	FileName    string        `json:"file_name,omitempty"`
	ContentType string        `json:"content_type,omitempty"`
	Description string        `json:"description,omitempty"`
}

// CreateItemOut — id созданного item-а + маскированные метаданные записанного значения.
type CreateItemOut struct {
	ID          string `json:"id"`
	ValueChars  int    `json:"value_chars"`
	ValueBytes  int    `json:"value_bytes"`
	ValueSha256 string `json:"value_sha256"`
}

func (s *Server) createItem(ctx context.Context, _ *mcp.CallToolRequest, in CreateItemIn) (*mcp.CallToolResult, CreateItemOut, error) {
	app, err := s.currentApp()
	if err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	if _, err = s.secretInCurrentApp(ctx, in.SecretID); err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	value, err := s.resolveValueSource(ctx, app.ID, in.ValueSource)
	if err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	id, err := s.api.ItemCreate(ctx, model.ItemCreateReq{
		SecretID:    in.SecretID,
		Active:      in.Active,
		Key:         in.Key,
		Value:       value,
		ValueFormat: in.ValueFormat,
		Encoding:    in.Encoding,
		FileName:    in.FileName,
		ContentType: in.ContentType,
		Description: in.Description,
	})
	if err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	masked := maskValue(value)

	return nil, CreateItemOut{
		ID:          id,
		ValueChars:  masked.Chars,
		ValueBytes:  masked.Bytes,
		ValueSha256: masked.Sha256,
	}, nil
}

type UpdateItemIn struct {
	ID          string         `json:"id" jsonschema:"id item-а"`
	SecretID    *string        `json:"secret_id,omitempty" jsonschema:"перенос в другой секрет текущего app"`
	Key         *string        `json:"key,omitempty"`
	ValueSource *ValueSourceIn `json:"value_source,omitempty" jsonschema:"новое значение; если не задано — значение не меняется"`
	Active      *bool          `json:"active,omitempty"`
	ValueFormat *string        `json:"value_format,omitempty"`
	Encoding    *string        `json:"encoding,omitempty"`
	FileName    *string        `json:"file_name,omitempty"`
	ContentType *string        `json:"content_type,omitempty"`
	Description *string        `json:"description,omitempty"`
}

func (s *Server) updateItem(ctx context.Context, _ *mcp.CallToolRequest, in UpdateItemIn) (*mcp.CallToolResult, StatusOut, error) {
	app, err := s.currentApp()
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	if _, err = s.itemInCurrentApp(ctx, in.ID); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	// перенос допустим только в секрет текущего app
	if in.SecretID != nil {
		if _, err = s.secretInCurrentApp(ctx, *in.SecretID); err != nil {
			return nil, StatusOut{}, s.toolErr(err)
		}
	}

	req := model.ItemUpdateReq{
		SecretID:    in.SecretID,
		Active:      in.Active,
		Key:         in.Key,
		ValueFormat: in.ValueFormat,
		Encoding:    in.Encoding,
		FileName:    in.FileName,
		ContentType: in.ContentType,
		Description: in.Description,
	}

	if in.ValueSource != nil {
		value, verr := s.resolveValueSource(ctx, app.ID, *in.ValueSource)
		if verr != nil {
			return nil, StatusOut{}, s.toolErr(verr)
		}
		req.Value = &value
	}

	if err = s.api.ItemUpdate(ctx, in.ID, req); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	return nil, StatusOut{OK: true}, nil
}

// ── ConfigItem ──────────────────────────────────────────

type CreateConfigItemIn struct {
	ConfigmapID string `json:"configmap_id" jsonschema:"id конфигмапа текущего app"`
	Key         string `json:"key" jsonschema:"ключ item-а"`
	Value       string `json:"value" jsonschema:"значение (несекретное)"`
	Active      *bool  `json:"active,omitempty"`
	ValueFormat string `json:"value_format,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description,omitempty"`
}

func (s *Server) createConfigItem(ctx context.Context, _ *mcp.CallToolRequest, in CreateConfigItemIn) (*mcp.CallToolResult, CreateOut, error) {
	if _, err := s.configMapInCurrentApp(ctx, in.ConfigmapID); err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	id, err := s.api.ConfigItemCreate(ctx, model.ConfigItemCreateReq{
		ConfigmapID: in.ConfigmapID,
		Active:      in.Active,
		Key:         in.Key,
		Value:       in.Value,
		ValueFormat: in.ValueFormat,
		Encoding:    in.Encoding,
		FileName:    in.FileName,
		ContentType: in.ContentType,
		Description: in.Description,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	return nil, CreateOut{ID: id}, nil
}

type UpdateConfigItemIn struct {
	ID          string  `json:"id" jsonschema:"id item-а конфигмапа"`
	ConfigmapID *string `json:"configmap_id,omitempty" jsonschema:"перенос в другой configmap текущего app"`
	Key         *string `json:"key,omitempty"`
	Value       *string `json:"value,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	ValueFormat *string `json:"value_format,omitempty"`
	Encoding    *string `json:"encoding,omitempty"`
	FileName    *string `json:"file_name,omitempty"`
	ContentType *string `json:"content_type,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (s *Server) updateConfigItem(ctx context.Context, _ *mcp.CallToolRequest, in UpdateConfigItemIn) (*mcp.CallToolResult, StatusOut, error) {
	item, err := s.api.ConfigItemGet(ctx, in.ID)
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	if _, err = s.configMapInCurrentApp(ctx, item.ConfigmapID); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	if in.ConfigmapID != nil {
		if _, err = s.configMapInCurrentApp(ctx, *in.ConfigmapID); err != nil {
			return nil, StatusOut{}, s.toolErr(err)
		}
	}

	err = s.api.ConfigItemUpdate(ctx, in.ID, model.ConfigItemUpdateReq{
		ConfigmapID: in.ConfigmapID,
		Active:      in.Active,
		Key:         in.Key,
		Value:       in.Value,
		ValueFormat: in.ValueFormat,
		Encoding:    in.Encoding,
		FileName:    in.FileName,
		ContentType: in.ContentType,
		Description: in.Description,
	})
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	return nil, StatusOut{OK: true}, nil
}
