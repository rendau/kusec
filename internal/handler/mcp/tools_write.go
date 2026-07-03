package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	configitemModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
)

// ── Регистрация ─────────────────────────────────────────

func (s *sessionServer) registerWriteTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "use_app",
		Description: "Выбрать текущий app (по id, slug_name или имени). Все операции записи (create/update секретов, конфигмапов и item-ов) разрешены только в текущем app.",
	}, s.useApp)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "current_app",
		Description: "Показать текущий app и имена значений, зарегистрированных в реестре сессии (сами значения не раскрываются).",
	}, s.currentAppTool)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "create_app",
		Description: "Создать app. Новый app автоматически становится текущим.",
	}, s.createApp)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "update_app",
		Description: "Обновить текущий app (только переданные поля).",
	}, s.updateApp)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "create_secret",
		Description: "Создать секрет в текущем app.",
	}, s.createSecret)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "update_secret",
		Description: "Обновить секрет текущего app (только переданные поля).",
	}, s.updateSecret)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "create_configmap",
		Description: "Создать configmap в текущем app.",
	}, s.createConfigMap)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "update_configmap",
		Description: "Обновить configmap текущего app (только переданные поля).",
	}, s.updateConfigMap)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "create_item",
		Description: "Создать item секрета в текущем app. Значение задаётся через value_source: generate — сгенерировать случайное (с сохранением в реестре сессии под name), reuse — переиспользовать сгенерированное ранее, copy_item — скопировать значение существующего item (из любого доступного app), literal — явное несекретное значение. Сгенерированные/скопированные значения агенту не показываются.",
	}, s.createItem)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "update_item",
		Description: "Обновить item секрета текущего app (только переданные поля). Значение — через value_source, как в create_item.",
	}, s.updateItem)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "create_config_item",
		Description: "Создать item конфигмапа в текущем app (значение несекретное, задаётся явно).",
	}, s.createConfigItem)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "update_config_item",
		Description: "Обновить item конфигмапа текущего app (только переданные поля).",
	}, s.updateConfigItem)
}

// ── App ─────────────────────────────────────────────────

type UseAppIn struct {
	App string `json:"app" jsonschema:"id, slug_name или имя app"`
}

func (s *sessionServer) useApp(ctx context.Context, req *mcpsdk.CallToolRequest, in UseAppIn) (*mcpsdk.CallToolResult, AppOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, AppOut{}, err
	}

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

func (s *sessionServer) currentAppTool(ctx context.Context, req *mcpsdk.CallToolRequest, _ struct{}) (*mcpsdk.CallToolResult, CurrentAppOut, error) {
	if _, err := s.toolCtx(ctx, req); err != nil {
		return nil, CurrentAppOut{}, err
	}

	app, err := s.currentApp()
	if err != nil {
		return nil, CurrentAppOut{ValueNames: []string{}}, nil
	}

	out := appToOut(app, 0)

	return nil, CurrentAppOut{App: &out, ValueNames: s.vault.names(app.Id)}, nil
}

type CreateAppIn struct {
	Namespace   string `json:"namespace" jsonschema:"k8s namespace приложения"`
	Name        string `json:"name" jsonschema:"человекочитаемое имя"`
	SlugName    string `json:"slug_name" jsonschema:"слаг (входит в имена k8s-объектов)"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type CreateOut struct {
	Id string `json:"id"`
}

func (s *sessionServer) createApp(ctx context.Context, req *mcpsdk.CallToolRequest, in CreateAppIn) (*mcpsdk.CallToolResult, CreateOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, CreateOut{}, err
	}

	newId, err := s.h.appUsecase.Create(ctx, &appModel.Edit{
		Active:      in.Active,
		Namespace:   &in.Namespace,
		Name:        &in.Name,
		SlugName:    &in.SlugName,
		Description: &in.Description,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	if app, gerr := s.h.appUsecase.Get(ctx, newId); gerr == nil {
		s.setCurrentApp(app)
	}

	return nil, CreateOut{Id: newId}, nil
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

func (s *sessionServer) updateApp(ctx context.Context, req *mcpsdk.CallToolRequest, in UpdateAppIn) (*mcpsdk.CallToolResult, StatusOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, StatusOut{}, err
	}

	app, err := s.currentApp()
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	err = s.h.appUsecase.Update(ctx, app.Id, &appModel.Edit{
		Active:      in.Active,
		Namespace:   in.Namespace,
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	// обновляем локальную копию текущего app
	if fresh, gerr := s.h.appUsecase.Get(ctx, app.Id); gerr == nil {
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

func (s *sessionServer) createSecret(ctx context.Context, req *mcpsdk.CallToolRequest, in CreateSecretIn) (*mcpsdk.CallToolResult, CreateOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, CreateOut{}, err
	}

	app, err := s.currentApp()
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	newId, err := s.h.secretUsecase.Create(ctx, &secretModel.Edit{
		AppId:       &app.Id,
		Active:      in.Active,
		SlugName:    &in.SlugName,
		Description: &in.Description,
		KubeType:    &in.KubeType,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	return nil, CreateOut{Id: newId}, nil
}

type UpdateSecretIn struct {
	Id          string  `json:"id" jsonschema:"id секрета"`
	Active      *bool   `json:"active,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
	KubeType    *string `json:"kube_type,omitempty"`
}

func (s *sessionServer) updateSecret(ctx context.Context, req *mcpsdk.CallToolRequest, in UpdateSecretIn) (*mcpsdk.CallToolResult, StatusOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, StatusOut{}, err
	}

	if _, err = s.secretInCurrentApp(ctx, in.Id); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	err = s.h.secretUsecase.Update(ctx, in.Id, &secretModel.Edit{
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

func (s *sessionServer) createConfigMap(ctx context.Context, req *mcpsdk.CallToolRequest, in CreateConfigMapIn) (*mcpsdk.CallToolResult, CreateOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, CreateOut{}, err
	}

	app, err := s.currentApp()
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	newId, err := s.h.configmapUsecase.Create(ctx, &configmapModel.Edit{
		AppId:       &app.Id,
		Active:      in.Active,
		SlugName:    &in.SlugName,
		Description: &in.Description,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	return nil, CreateOut{Id: newId}, nil
}

type UpdateConfigMapIn struct {
	Id          string  `json:"id" jsonschema:"id конфигмапа"`
	Active      *bool   `json:"active,omitempty"`
	SlugName    *string `json:"slug_name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (s *sessionServer) updateConfigMap(ctx context.Context, req *mcpsdk.CallToolRequest, in UpdateConfigMapIn) (*mcpsdk.CallToolResult, StatusOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, StatusOut{}, err
	}

	if _, err = s.configMapInCurrentApp(ctx, in.Id); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	err = s.h.configmapUsecase.Update(ctx, in.Id, &configmapModel.Edit{
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
	SecretId    string        `json:"secret_id" jsonschema:"id секрета текущего app"`
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
	Id          string `json:"id"`
	ValueChars  int    `json:"value_chars"`
	ValueBytes  int    `json:"value_bytes"`
	ValueSha256 string `json:"value_sha256"`
}

func (s *sessionServer) createItem(ctx context.Context, req *mcpsdk.CallToolRequest, in CreateItemIn) (*mcpsdk.CallToolResult, CreateItemOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, CreateItemOut{}, err
	}

	app, err := s.currentApp()
	if err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	if _, err = s.secretInCurrentApp(ctx, in.SecretId); err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	value, err := s.resolveValueSource(ctx, app.Id, in.ValueSource)
	if err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	newId, err := s.h.itemUsecase.Create(ctx, &itemModel.Edit{
		SecretId:    &in.SecretId,
		Active:      in.Active,
		Key:         &in.Key,
		Value:       &value,
		ValueFormat: optStr(in.ValueFormat),
		Encoding:    optStr(in.Encoding),
		FileName:    optStr(in.FileName),
		ContentType: optStr(in.ContentType),
		Description: &in.Description,
	})
	if err != nil {
		return nil, CreateItemOut{}, s.toolErr(err)
	}

	masked := maskValue(value)

	return nil, CreateItemOut{
		Id:          newId,
		ValueChars:  masked.Chars,
		ValueBytes:  masked.Bytes,
		ValueSha256: masked.Sha256,
	}, nil
}

type UpdateItemIn struct {
	Id          string         `json:"id" jsonschema:"id item-а"`
	SecretId    *string        `json:"secret_id,omitempty" jsonschema:"перенос в другой секрет текущего app"`
	Key         *string        `json:"key,omitempty"`
	ValueSource *ValueSourceIn `json:"value_source,omitempty" jsonschema:"новое значение; если не задано — значение не меняется"`
	Active      *bool          `json:"active,omitempty"`
	ValueFormat *string        `json:"value_format,omitempty"`
	Encoding    *string        `json:"encoding,omitempty"`
	FileName    *string        `json:"file_name,omitempty"`
	ContentType *string        `json:"content_type,omitempty"`
	Description *string        `json:"description,omitempty"`
}

func (s *sessionServer) updateItem(ctx context.Context, req *mcpsdk.CallToolRequest, in UpdateItemIn) (*mcpsdk.CallToolResult, StatusOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, StatusOut{}, err
	}

	app, err := s.currentApp()
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	if _, err = s.itemInCurrentApp(ctx, in.Id); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	// перенос допустим только в секрет текущего app
	if in.SecretId != nil {
		if _, err = s.secretInCurrentApp(ctx, *in.SecretId); err != nil {
			return nil, StatusOut{}, s.toolErr(err)
		}
	}

	edit := &itemModel.Edit{
		SecretId:    in.SecretId,
		Active:      in.Active,
		Key:         in.Key,
		ValueFormat: in.ValueFormat,
		Encoding:    in.Encoding,
		FileName:    in.FileName,
		ContentType: in.ContentType,
		Description: in.Description,
	}

	if in.ValueSource != nil {
		value, verr := s.resolveValueSource(ctx, app.Id, *in.ValueSource)
		if verr != nil {
			return nil, StatusOut{}, s.toolErr(verr)
		}
		edit.Value = &value
	}

	if err = s.h.itemUsecase.Update(ctx, in.Id, edit); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	return nil, StatusOut{OK: true}, nil
}

// ── ConfigItem ──────────────────────────────────────────

type CreateConfigItemIn struct {
	ConfigmapId string `json:"configmap_id" jsonschema:"id конфигмапа текущего app"`
	Key         string `json:"key" jsonschema:"ключ item-а"`
	Value       string `json:"value" jsonschema:"значение (несекретное)"`
	Active      *bool  `json:"active,omitempty"`
	ValueFormat string `json:"value_format,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description,omitempty"`
}

func (s *sessionServer) createConfigItem(ctx context.Context, req *mcpsdk.CallToolRequest, in CreateConfigItemIn) (*mcpsdk.CallToolResult, CreateOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, CreateOut{}, err
	}

	if _, err = s.configMapInCurrentApp(ctx, in.ConfigmapId); err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	newId, err := s.h.configitemUsecase.Create(ctx, &configitemModel.Edit{
		ConfigMapId: &in.ConfigmapId,
		Active:      in.Active,
		Key:         &in.Key,
		Value:       &in.Value,
		ValueFormat: optStr(in.ValueFormat),
		Encoding:    optStr(in.Encoding),
		FileName:    optStr(in.FileName),
		ContentType: optStr(in.ContentType),
		Description: &in.Description,
	})
	if err != nil {
		return nil, CreateOut{}, s.toolErr(err)
	}

	return nil, CreateOut{Id: newId}, nil
}

type UpdateConfigItemIn struct {
	Id          string  `json:"id" jsonschema:"id item-а конфигмапа"`
	ConfigmapId *string `json:"configmap_id,omitempty" jsonschema:"перенос в другой configmap текущего app"`
	Key         *string `json:"key,omitempty"`
	Value       *string `json:"value,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	ValueFormat *string `json:"value_format,omitempty"`
	Encoding    *string `json:"encoding,omitempty"`
	FileName    *string `json:"file_name,omitempty"`
	ContentType *string `json:"content_type,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (s *sessionServer) updateConfigItem(ctx context.Context, req *mcpsdk.CallToolRequest, in UpdateConfigItemIn) (*mcpsdk.CallToolResult, StatusOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, StatusOut{}, err
	}

	item, err := s.h.configitemUsecase.Get(ctx, in.Id)
	if err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	if _, err = s.configMapInCurrentApp(ctx, item.ConfigMapId); err != nil {
		return nil, StatusOut{}, s.toolErr(err)
	}

	if in.ConfigmapId != nil {
		if _, err = s.configMapInCurrentApp(ctx, *in.ConfigmapId); err != nil {
			return nil, StatusOut{}, s.toolErr(err)
		}
	}

	err = s.h.configitemUsecase.Update(ctx, in.Id, &configitemModel.Edit{
		ConfigMapId: in.ConfigmapId,
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
