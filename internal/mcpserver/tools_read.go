package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	"github.com/mechta-market/kusec/internal/mcpserver/client/model"
)

// ── Общие вход/выход ────────────────────────────────────

type PageIn struct {
	Page     int64 `json:"page,omitempty" jsonschema:"номер страницы, начиная с 0"`
	PageSize int64 `json:"page_size,omitempty" jsonschema:"размер страницы, по умолчанию 100"`
}

func (p PageIn) listParams() model.ListParams {
	return model.ListParams{Page: p.Page, PageSize: p.PageSize, WithTotalCount: true}
}

type PaginationOut struct {
	Page       int64 `json:"page"`
	PageSize   int64 `json:"page_size"`
	TotalCount int64 `json:"total_count"`
}

func paginationToOut(v model.PaginationInfo) PaginationOut {
	return PaginationOut{
		Page:       int64(v.Page),
		PageSize:   int64(v.PageSize),
		TotalCount: int64(v.TotalCount),
	}
}

type AppOut struct {
	ID          string `json:"id"`
	Active      bool   `json:"active"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	SlugName    string `json:"slug_name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func appToOut(v model.App, _ int) AppOut {
	return AppOut{
		ID:          v.ID,
		Active:      v.Active,
		Namespace:   v.Namespace,
		Name:        v.Name,
		SlugName:    v.SlugName,
		Description: v.Description,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

type SecretOut struct {
	ID             string `json:"id"`
	AppID          string `json:"app_id"`
	Active         bool   `json:"active"`
	SlugName       string `json:"slug_name"`
	Description    string `json:"description"`
	KubeSecretName string `json:"kube_secret_name"`
	KubeType       string `json:"kube_type,omitempty"`
	ExactSlug      bool   `json:"exact_slug,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

func secretToOut(v model.Secret, _ int) SecretOut {
	return SecretOut{
		ID:             v.ID,
		AppID:          v.AppID,
		Active:         v.Active,
		SlugName:       v.SlugName,
		Description:    v.Description,
		KubeSecretName: v.KubeSecretName,
		KubeType:       v.KubeType,
		ExactSlug:      v.ExactSlug,
		CreatedAt:      v.CreatedAt,
		UpdatedAt:      v.UpdatedAt,
	}
}

type ConfigMapOut struct {
	ID                string `json:"id"`
	AppID             string `json:"app_id"`
	Active            bool   `json:"active"`
	SlugName          string `json:"slug_name"`
	Description       string `json:"description"`
	KubeConfigmapName string `json:"kube_configmap_name"`
	ExactSlug         bool   `json:"exact_slug,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

func configMapToOut(v model.ConfigMap, _ int) ConfigMapOut {
	return ConfigMapOut{
		ID:                v.ID,
		AppID:             v.AppID,
		Active:            v.Active,
		SlugName:          v.SlugName,
		Description:       v.Description,
		KubeConfigmapName: v.KubeConfigmapName,
		ExactSlug:         v.ExactSlug,
		CreatedAt:         v.CreatedAt,
		UpdatedAt:         v.UpdatedAt,
	}
}

// ItemOut — item секрета без значения: вместо value только его метаданные.
type ItemOut struct {
	ID          string `json:"id"`
	SecretID    string `json:"secret_id"`
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
func (s *Server) maskItem(v model.Item, _ int) ItemOut {
	s.vault.markSeen(v.Value)
	masked := maskValue(v.Value)

	return ItemOut{
		ID:          v.ID,
		SecretID:    v.SecretID,
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
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

// ConfigItemOut — item конфигмапа; значения конфигмапов не секретны и выдаются как есть.
type ConfigItemOut struct {
	ID          string `json:"id"`
	ConfigmapID string `json:"configmap_id"`
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

func configItemToOut(v model.ConfigItem, _ int) ConfigItemOut {
	return ConfigItemOut{
		ID:          v.ID,
		ConfigmapID: v.ConfigmapID,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: v.Description,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

// ── Регистрация ─────────────────────────────────────────

func (s *Server) registerReadTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_app",
		Description: "Список приложений (app) kusec с фильтрами и пагинацией.",
	}, s.listApp)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_app",
		Description: "Получить app по id.",
	}, s.getApp)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_secret",
		Description: "Список секретов (k8s Secret) с фильтрами и пагинацией.",
	}, s.listSecret)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_secret",
		Description: "Получить секрет по id (метаданные, без значений item-ов).",
	}, s.getSecret)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_configmap",
		Description: "Список конфигмапов (k8s ConfigMap) с фильтрами и пагинацией.",
	}, s.listConfigMap)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_configmap",
		Description: "Получить configmap по id.",
	}, s.getConfigMap)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_item",
		Description: "Список item-ов секретов. Значения замаскированы: вместо value — длина в символах/байтах и усечённый sha256 для сравнения.",
	}, s.listItem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_item",
		Description: "Получить item секрета по id. Значение замаскировано (длина + усечённый sha256).",
	}, s.getItem)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_config_item",
		Description: "Список item-ов конфигмапов (значения не секретны и видны полностью).",
	}, s.listConfigItem)

	mcp.AddTool(srv, &mcp.Tool{
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

func (s *Server) listApp(ctx context.Context, _ *mcp.CallToolRequest, in ListAppIn) (*mcp.CallToolResult, ListAppOut, error) {
	req := model.AppListReq{ListParams: in.listParams(), Active: in.Active}
	if in.Namespace != "" {
		req.Namespace = &in.Namespace
	}
	if in.Search != "" {
		req.Search = &in.Search
	}

	rep, err := s.api.AppList(ctx, req)
	if err != nil {
		return nil, ListAppOut{}, s.toolErr(err)
	}

	return nil, ListAppOut{
		PaginationInfo: paginationToOut(rep.PaginationInfo),
		Results:        lo.Map(rep.Results, appToOut),
	}, nil
}

type GetByIdIn struct {
	ID string `json:"id" jsonschema:"id сущности"`
}

func (s *Server) getApp(ctx context.Context, _ *mcp.CallToolRequest, in GetByIdIn) (*mcp.CallToolResult, AppOut, error) {
	rep, err := s.api.AppGet(ctx, in.ID)
	if err != nil {
		return nil, AppOut{}, s.toolErr(err)
	}

	return nil, appToOut(rep, 0), nil
}

// ── Secret ──────────────────────────────────────────────

type ListSecretIn struct {
	PageIn
	AppID  string `json:"app_id,omitempty" jsonschema:"фильтр по app"`
	Active *bool  `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search string `json:"search,omitempty" jsonschema:"поиск по слагу/описанию"`
}

type ListSecretOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []SecretOut   `json:"results"`
}

func (s *Server) listSecret(ctx context.Context, _ *mcp.CallToolRequest, in ListSecretIn) (*mcp.CallToolResult, ListSecretOut, error) {
	req := model.SecretListReq{ListParams: in.listParams(), Active: in.Active}
	if in.AppID != "" {
		req.AppID = &in.AppID
	}
	if in.Search != "" {
		req.Search = &in.Search
	}

	rep, err := s.api.SecretList(ctx, req)
	if err != nil {
		return nil, ListSecretOut{}, s.toolErr(err)
	}

	return nil, ListSecretOut{
		PaginationInfo: paginationToOut(rep.PaginationInfo),
		Results:        lo.Map(rep.Results, secretToOut),
	}, nil
}

func (s *Server) getSecret(ctx context.Context, _ *mcp.CallToolRequest, in GetByIdIn) (*mcp.CallToolResult, SecretOut, error) {
	rep, err := s.api.SecretGet(ctx, in.ID)
	if err != nil {
		return nil, SecretOut{}, s.toolErr(err)
	}

	return nil, secretToOut(rep, 0), nil
}

// ── ConfigMap ───────────────────────────────────────────

type ListConfigMapIn struct {
	PageIn
	AppID  string `json:"app_id,omitempty" jsonschema:"фильтр по app"`
	Active *bool  `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search string `json:"search,omitempty" jsonschema:"поиск по слагу/описанию"`
}

type ListConfigMapOut struct {
	PaginationInfo PaginationOut  `json:"pagination_info"`
	Results        []ConfigMapOut `json:"results"`
}

func (s *Server) listConfigMap(ctx context.Context, _ *mcp.CallToolRequest, in ListConfigMapIn) (*mcp.CallToolResult, ListConfigMapOut, error) {
	req := model.ConfigMapListReq{ListParams: in.listParams(), Active: in.Active}
	if in.AppID != "" {
		req.AppID = &in.AppID
	}
	if in.Search != "" {
		req.Search = &in.Search
	}

	rep, err := s.api.ConfigMapList(ctx, req)
	if err != nil {
		return nil, ListConfigMapOut{}, s.toolErr(err)
	}

	return nil, ListConfigMapOut{
		PaginationInfo: paginationToOut(rep.PaginationInfo),
		Results:        lo.Map(rep.Results, configMapToOut),
	}, nil
}

func (s *Server) getConfigMap(ctx context.Context, _ *mcp.CallToolRequest, in GetByIdIn) (*mcp.CallToolResult, ConfigMapOut, error) {
	rep, err := s.api.ConfigMapGet(ctx, in.ID)
	if err != nil {
		return nil, ConfigMapOut{}, s.toolErr(err)
	}

	return nil, configMapToOut(rep, 0), nil
}

// ── Item ────────────────────────────────────────────────

type ListItemIn struct {
	PageIn
	SecretID  string   `json:"secret_id,omitempty" jsonschema:"фильтр по секрету"`
	SecretIDs []string `json:"secret_ids,omitempty" jsonschema:"выборка по нескольким секретам за один запрос"`
	Active    *bool    `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search    string   `json:"search,omitempty" jsonschema:"поиск по ключу/описанию"`
}

type ListItemOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []ItemOut     `json:"results"`
}

func (s *Server) listItem(ctx context.Context, _ *mcp.CallToolRequest, in ListItemIn) (*mcp.CallToolResult, ListItemOut, error) {
	req := model.ItemListReq{ListParams: in.listParams(), SecretIDs: in.SecretIDs, Active: in.Active}
	if in.SecretID != "" {
		req.SecretID = &in.SecretID
	}
	if in.Search != "" {
		req.Search = &in.Search
	}

	rep, err := s.api.ItemList(ctx, req)
	if err != nil {
		return nil, ListItemOut{}, s.toolErr(err)
	}

	return nil, ListItemOut{
		PaginationInfo: paginationToOut(rep.PaginationInfo),
		Results:        lo.Map(rep.Results, s.maskItem),
	}, nil
}

func (s *Server) getItem(ctx context.Context, _ *mcp.CallToolRequest, in GetByIdIn) (*mcp.CallToolResult, ItemOut, error) {
	rep, err := s.api.ItemGet(ctx, in.ID)
	if err != nil {
		return nil, ItemOut{}, s.toolErr(err)
	}

	return nil, s.maskItem(rep, 0), nil
}

// ── ConfigItem ──────────────────────────────────────────

type ListConfigItemIn struct {
	PageIn
	ConfigmapID  string   `json:"configmap_id,omitempty" jsonschema:"фильтр по конфигмапу"`
	ConfigmapIDs []string `json:"configmap_ids,omitempty" jsonschema:"выборка по нескольким конфигмапам за один запрос"`
	Active       *bool    `json:"active,omitempty" jsonschema:"фильтр по активности"`
	Search       string   `json:"search,omitempty" jsonschema:"поиск по ключу/описанию"`
}

type ListConfigItemOut struct {
	PaginationInfo PaginationOut   `json:"pagination_info"`
	Results        []ConfigItemOut `json:"results"`
}

func (s *Server) listConfigItem(ctx context.Context, _ *mcp.CallToolRequest, in ListConfigItemIn) (*mcp.CallToolResult, ListConfigItemOut, error) {
	req := model.ConfigItemListReq{ListParams: in.listParams(), ConfigmapIDs: in.ConfigmapIDs, Active: in.Active}
	if in.ConfigmapID != "" {
		req.ConfigmapID = &in.ConfigmapID
	}
	if in.Search != "" {
		req.Search = &in.Search
	}

	rep, err := s.api.ConfigItemList(ctx, req)
	if err != nil {
		return nil, ListConfigItemOut{}, s.toolErr(err)
	}

	return nil, ListConfigItemOut{
		PaginationInfo: paginationToOut(rep.PaginationInfo),
		Results:        lo.Map(rep.Results, configItemToOut),
	}, nil
}

func (s *Server) getConfigItem(ctx context.Context, _ *mcp.CallToolRequest, in GetByIdIn) (*mcp.CallToolResult, ConfigItemOut, error) {
	rep, err := s.api.ConfigItemGet(ctx, in.ID)
	if err != nil {
		return nil, ConfigItemOut{}, s.toolErr(err)
	}

	return nil, configItemToOut(rep, 0), nil
}
