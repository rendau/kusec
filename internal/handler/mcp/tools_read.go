package mcp

import (
	"context"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	commonModel "github.com/mechta-market/kusec/internal/domain/common/model"
	configitemModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
)

// ── Общие вход/выход ────────────────────────────────────

type PageIn struct {
	Page     int64 `json:"page,omitempty" jsonschema:"номер страницы, начиная с 0"`
	PageSize int64 `json:"page_size,omitempty" jsonschema:"размер страницы, по умолчанию 100"`
}

func (p PageIn) listParams() commonModel.ListParams {
	if p.PageSize <= 0 {
		p.PageSize = 100
	}
	return commonModel.ListParams{Page: p.Page, PageSize: p.PageSize, WithTotalCount: true}
}

type PaginationOut struct {
	Page       int64 `json:"page"`
	PageSize   int64 `json:"page_size"`
	TotalCount int64 `json:"total_count"`
}

func paginationOut(lp commonModel.ListParams, totalCount int64) PaginationOut {
	return PaginationOut{Page: lp.Page, PageSize: lp.PageSize, TotalCount: totalCount}
}

func timeOut(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.Format(time.RFC3339)
}

func optStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

type AppOut struct {
	Id          string `json:"id"`
	Active      bool   `json:"active"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	SlugName    string `json:"slug_name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func appToOut(v *appModel.Main, _ int) AppOut {
	return AppOut{
		Id:          v.Id,
		Active:      v.Active,
		Namespace:   v.Namespace,
		Name:        v.Name,
		SlugName:    v.SlugName,
		Description: v.Description,
		CreatedAt:   timeOut(v.CreatedAt),
		UpdatedAt:   timeOut(v.UpdatedAt),
	}
}

type SecretOut struct {
	Id             string `json:"id"`
	AppId          string `json:"app_id"`
	Active         bool   `json:"active"`
	SlugName       string `json:"slug_name"`
	Description    string `json:"description"`
	KubeSecretName string `json:"kube_secret_name"`
	KubeType       string `json:"kube_type,omitempty"`
	ExactSlug      bool   `json:"exact_slug,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

func secretToOut(v *secretModel.Main, _ int) SecretOut {
	return SecretOut{
		Id:             v.Id,
		AppId:          v.AppId,
		Active:         v.Active,
		SlugName:       v.SlugName,
		Description:    v.Description,
		KubeSecretName: v.KubeSecretName,
		KubeType:       v.KubeType,
		ExactSlug:      v.ExactSlug,
		CreatedAt:      timeOut(v.CreatedAt),
		UpdatedAt:      timeOut(v.UpdatedAt),
	}
}

type ConfigMapOut struct {
	Id                string `json:"id"`
	AppId             string `json:"app_id"`
	Active            bool   `json:"active"`
	SlugName          string `json:"slug_name"`
	Description       string `json:"description"`
	KubeConfigmapName string `json:"kube_configmap_name"`
	ExactSlug         bool   `json:"exact_slug,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

func configMapToOut(v *configmapModel.Main, _ int) ConfigMapOut {
	return ConfigMapOut{
		Id:                v.Id,
		AppId:             v.AppId,
		Active:            v.Active,
		SlugName:          v.SlugName,
		Description:       v.Description,
		KubeConfigmapName: v.KubeConfigMapName,
		ExactSlug:         v.ExactSlug,
		CreatedAt:         timeOut(v.CreatedAt),
		UpdatedAt:         timeOut(v.UpdatedAt),
	}
}

// ItemOut — item секрета без значения: вместо value только его метаданные.
type ItemOut struct {
	Id          string `json:"id"`
	SecretId    string `json:"secret_id"`
	Active      bool   `json:"active"`
	Key         string `json:"key"`
	ValueChars  int    `json:"value_chars" jsonschema:"длина значения в символах (само значение агенту не выдаётся)"`
	ValueBytes  int    `json:"value_bytes" jsonschema:"длина значения в байтах"`
	ValueSha256 string `json:"value_sha256" jsonschema:"усечённый sha256 значения — для сравнения значений между собой"`
	ValueFormat string `json:"value_format,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// maskItem конвертирует item в безопасный вид и запоминает значение для скраба ошибок.
func (s *sessionServer) maskItem(v *itemModel.Main, _ int) ItemOut {
	s.vault.markSeen(v.Value)
	masked := maskValue(v.Value)

	return ItemOut{
		Id:          v.Id,
		SecretId:    v.SecretId,
		Active:      v.Active,
		Key:         v.Key,
		ValueChars:  masked.Chars,
		ValueBytes:  masked.Bytes,
		ValueSha256: masked.Sha256,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: v.Description,
		CreatedAt:   timeOut(v.CreatedAt),
		UpdatedAt:   timeOut(v.UpdatedAt),
	}
}

// ConfigItemOut — item конфигмапа; значения конфигмапов не секретны и выдаются как есть.
type ConfigItemOut struct {
	Id          string `json:"id"`
	ConfigmapId string `json:"configmap_id"`
	Active      bool   `json:"active"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueFormat string `json:"value_format,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func configItemToOut(v *configitemModel.Main, _ int) ConfigItemOut {
	return ConfigItemOut{
		Id:          v.Id,
		ConfigmapId: v.ConfigMapId,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: v.Description,
		CreatedAt:   timeOut(v.CreatedAt),
		UpdatedAt:   timeOut(v.UpdatedAt),
	}
}

// ── Регистрация ─────────────────────────────────────────

func (s *sessionServer) registerReadTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_app",
		Description: "Список приложений (app) kusec с фильтрами и пагинацией.",
	}, s.listApp)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_app",
		Description: "Получить app по id.",
	}, s.getApp)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_secret",
		Description: "Список секретов (k8s Secret) с фильтрами и пагинацией.",
	}, s.listSecret)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_secret",
		Description: "Получить секрет по id (метаданные, без значений item-ов).",
	}, s.getSecret)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_configmap",
		Description: "Список конфигмапов (k8s ConfigMap) с фильтрами и пагинацией.",
	}, s.listConfigMap)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_configmap",
		Description: "Получить configmap по id.",
	}, s.getConfigMap)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_item",
		Description: "Список item-ов секретов. Значения замаскированы: вместо value — длина в символах/байтах и усечённый sha256 для сравнения.",
	}, s.listItem)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_item",
		Description: "Получить item секрета по id. Значение замаскировано (длина + усечённый sha256).",
	}, s.getItem)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_config_item",
		Description: "Список item-ов конфигмапов (значения не секретны и видны полностью).",
	}, s.listConfigItem)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_config_item",
		Description: "Получить item конфигмапа по id.",
	}, s.getConfigItem)
}

// ── App ─────────────────────────────────────────────────

type ListAppIn struct {
	PageIn
	Active    *bool  `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Namespace string `json:"namespace,omitempty" jsonschema:"фильтр по k8s namespace"`
	Search    string `json:"search,omitempty" jsonschema:"поиск по названию/слагу"`
}

type ListAppOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []AppOut      `json:"results"`
}

func (s *sessionServer) listApp(ctx context.Context, req *mcpsdk.CallToolRequest, in ListAppIn) (*mcpsdk.CallToolResult, ListAppOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListAppOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.appUsecase.List(ctx, &appModel.ListReq{
		ListParams: lp,
		Active:     in.Active,
		Namespace:  optStr(in.Namespace),
		Search:     optStr(in.Search),
	})
	if err != nil {
		return nil, ListAppOut{}, s.toolErr(err)
	}

	return nil, ListAppOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, appToOut),
	}, nil
}

type GetByIdIn struct {
	Id string `json:"id" jsonschema:"id сущности"`
}

func (s *sessionServer) getApp(ctx context.Context, req *mcpsdk.CallToolRequest, in GetByIdIn) (*mcpsdk.CallToolResult, AppOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, AppOut{}, err
	}

	item, err := s.h.appUsecase.Get(ctx, in.Id)
	if err != nil {
		return nil, AppOut{}, s.toolErr(err)
	}

	return nil, appToOut(item, 0), nil
}

// ── Secret ──────────────────────────────────────────────

type ListSecretIn struct {
	PageIn
	AppId  string `json:"app_id,omitempty" jsonschema:"фильтр по app"`
	Active *bool  `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search string `json:"search,omitempty" jsonschema:"поиск по слагу/описанию"`
}

type ListSecretOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []SecretOut   `json:"results"`
}

func (s *sessionServer) listSecret(ctx context.Context, req *mcpsdk.CallToolRequest, in ListSecretIn) (*mcpsdk.CallToolResult, ListSecretOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListSecretOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.secretUsecase.List(ctx, &secretModel.ListReq{
		ListParams: lp,
		AppId:      optStr(in.AppId),
		Active:     in.Active,
		Search:     optStr(in.Search),
	})
	if err != nil {
		return nil, ListSecretOut{}, s.toolErr(err)
	}

	return nil, ListSecretOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, secretToOut),
	}, nil
}

func (s *sessionServer) getSecret(ctx context.Context, req *mcpsdk.CallToolRequest, in GetByIdIn) (*mcpsdk.CallToolResult, SecretOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, SecretOut{}, err
	}

	item, err := s.h.secretUsecase.Get(ctx, in.Id)
	if err != nil {
		return nil, SecretOut{}, s.toolErr(err)
	}

	return nil, secretToOut(item, 0), nil
}

// ── ConfigMap ───────────────────────────────────────────

type ListConfigMapIn struct {
	PageIn
	AppId  string `json:"app_id,omitempty" jsonschema:"фильтр по app"`
	Active *bool  `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search string `json:"search,omitempty" jsonschema:"поиск по слагу/описанию"`
}

type ListConfigMapOut struct {
	PaginationInfo PaginationOut  `json:"pagination_info"`
	Results        []ConfigMapOut `json:"results"`
}

func (s *sessionServer) listConfigMap(ctx context.Context, req *mcpsdk.CallToolRequest, in ListConfigMapIn) (*mcpsdk.CallToolResult, ListConfigMapOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListConfigMapOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.configmapUsecase.List(ctx, &configmapModel.ListReq{
		ListParams: lp,
		AppId:      optStr(in.AppId),
		Active:     in.Active,
		Search:     optStr(in.Search),
	})
	if err != nil {
		return nil, ListConfigMapOut{}, s.toolErr(err)
	}

	return nil, ListConfigMapOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, configMapToOut),
	}, nil
}

func (s *sessionServer) getConfigMap(ctx context.Context, req *mcpsdk.CallToolRequest, in GetByIdIn) (*mcpsdk.CallToolResult, ConfigMapOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ConfigMapOut{}, err
	}

	item, err := s.h.configmapUsecase.Get(ctx, in.Id)
	if err != nil {
		return nil, ConfigMapOut{}, s.toolErr(err)
	}

	return nil, configMapToOut(item, 0), nil
}

// ── Item ────────────────────────────────────────────────

type ListItemIn struct {
	PageIn
	SecretId  string   `json:"secret_id,omitempty" jsonschema:"фильтр по секрету"`
	SecretIds []string `json:"secret_ids,omitempty" jsonschema:"выборка по нескольким секретам за один запрос"`
	Active    *bool    `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search    string   `json:"search,omitempty" jsonschema:"поиск по ключу/описанию"`
}

type ListItemOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []ItemOut     `json:"results"`
}

func (s *sessionServer) listItem(ctx context.Context, req *mcpsdk.CallToolRequest, in ListItemIn) (*mcpsdk.CallToolResult, ListItemOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListItemOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.itemUsecase.List(ctx, &itemModel.ListReq{
		ListParams: lp,
		SecretId:   optStr(in.SecretId),
		SecretIds:  in.SecretIds,
		Active:     in.Active,
		Search:     optStr(in.Search),
	})
	if err != nil {
		return nil, ListItemOut{}, s.toolErr(err)
	}

	return nil, ListItemOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, s.maskItem),
	}, nil
}

func (s *sessionServer) getItem(ctx context.Context, req *mcpsdk.CallToolRequest, in GetByIdIn) (*mcpsdk.CallToolResult, ItemOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ItemOut{}, err
	}

	item, err := s.h.itemUsecase.Get(ctx, in.Id)
	if err != nil {
		return nil, ItemOut{}, s.toolErr(err)
	}

	return nil, s.maskItem(item, 0), nil
}

// ── ConfigItem ──────────────────────────────────────────

type ListConfigItemIn struct {
	PageIn
	ConfigmapId  string   `json:"configmap_id,omitempty" jsonschema:"фильтр по конфигмапу"`
	ConfigmapIds []string `json:"configmap_ids,omitempty" jsonschema:"выборка по нескольким конфигмапам за один запрос"`
	Active       *bool    `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search       string   `json:"search,omitempty" jsonschema:"поиск по ключу/описанию"`
}

type ListConfigItemOut struct {
	PaginationInfo PaginationOut   `json:"pagination_info"`
	Results        []ConfigItemOut `json:"results"`
}

func (s *sessionServer) listConfigItem(ctx context.Context, req *mcpsdk.CallToolRequest, in ListConfigItemIn) (*mcpsdk.CallToolResult, ListConfigItemOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ListConfigItemOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.configitemUsecase.List(ctx, &configitemModel.ListReq{
		ListParams:   lp,
		ConfigMapId:  optStr(in.ConfigmapId),
		ConfigMapIds: in.ConfigmapIds,
		Active:       in.Active,
		Search:       optStr(in.Search),
	})
	if err != nil {
		return nil, ListConfigItemOut{}, s.toolErr(err)
	}

	return nil, ListConfigItemOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, configItemToOut),
	}, nil
}

func (s *sessionServer) getConfigItem(ctx context.Context, req *mcpsdk.CallToolRequest, in GetByIdIn) (*mcpsdk.CallToolResult, ConfigItemOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ConfigItemOut{}, err
	}

	item, err := s.h.configitemUsecase.Get(ctx, in.Id)
	if err != nil {
		return nil, ConfigItemOut{}, s.toolErr(err)
	}

	return nil, configItemToOut(item, 0), nil
}
