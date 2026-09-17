package mcp

import (
	"context"
	"fmt"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
)

// ── Регистрация ─────────────────────────────────────────

func (s *sessionServer) registerMonitoringTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "audit_list",
		Description: "Аудит изменений kusec: кто, когда и что менял (create/update/delete/activate/deactivate/import/sync). Значения секретов не раскрываются — только HMAC-отпечатки и размеры. Фильтры по app, объекту, действию, актору и окну времени (RFC3339); новые записи первыми.",
	}, s.auditList)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "sync_run_list",
		Description: "Журнал запусков синхронизации kusec → Kubernetes: статус (running|ok|partial|error), актор, длительность. Укажи id — вернётся один запуск с затронутыми объектами (op, имена изменившихся ключей, отпечаток содержимого).",
	}, s.syncRunList)
}

// parseTimePtr — RFC3339 → *time.Time; пустая строка → nil.
func parseTimePtr(field, value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%s: ожидается RFC3339 (например 2026-09-17T10:00:00Z): %w", field, err)
	}
	return &parsed, nil
}

func optTimeOut(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.Format(time.RFC3339)
}

// ── audit_list ──────────────────────────────────────────

type AuditListIn struct {
	PageIn
	AppId        string `json:"app_id,omitempty" jsonschema:"фильтр по app"`
	Namespace    string `json:"namespace,omitempty" jsonschema:"фильтр по k8s namespace"`
	AppSlug      string `json:"app_slug,omitempty" jsonschema:"фильтр по слагу app"`
	KubeName     string `json:"kube_name,omitempty" jsonschema:"фильтр по имени k8s-объекта (например kusec-caravan-main)"`
	EntityType   string `json:"entity_type,omitempty" jsonschema:"app | secret | item | configmap | config_item | api_key | usr | sync_run"`
	EntityId     string `json:"entity_id,omitempty" jsonschema:"фильтр по id сущности"`
	Action       string `json:"action,omitempty" jsonschema:"create | update | delete | activate | deactivate | sync | import"`
	ActorUsrId   *int64 `json:"actor_usr_id,omitempty" jsonschema:"фильтр по пользователю"`
	Key          string `json:"key,omitempty" jsonschema:"фильтр по ключу item/config_item"`
	BatchId      string `json:"batch_id,omitempty" jsonschema:"фильтр по batch id (каскадное удаление, импорт, sync)"`
	CreatedAtGte string `json:"created_at_gte,omitempty" jsonschema:"нижняя граница времени (RFC3339)"`
	CreatedAtLt  string `json:"created_at_lt,omitempty" jsonschema:"верхняя граница времени (RFC3339)"`
}

type AuditChangeOut struct {
	Field     string  `json:"field"`
	Old       *string `json:"old,omitempty"`
	New       *string `json:"new,omitempty"`
	OldHash   string  `json:"old_hash,omitempty" jsonschema:"HMAC-отпечаток старого значения (для секретов)"`
	NewHash   string  `json:"new_hash,omitempty"`
	OldSize   *int64  `json:"old_size,omitempty"`
	NewSize   *int64  `json:"new_size,omitempty"`
	Truncated bool    `json:"truncated,omitempty"`
}

type AuditOut struct {
	Id            int64            `json:"id"`
	CreatedAt     string           `json:"created_at"`
	ActorUsrId    *int64           `json:"actor_usr_id,omitempty"`
	ActorApiKeyId *string          `json:"actor_api_key_id,omitempty"`
	ActorName     string           `json:"actor_name"`
	Source        string           `json:"source"`
	EntityType    string           `json:"entity_type"`
	EntityId      string           `json:"entity_id"`
	AppId         *string          `json:"app_id,omitempty"`
	Namespace     string           `json:"namespace,omitempty"`
	AppSlug       string           `json:"app_slug,omitempty"`
	KubeKind      string           `json:"kube_kind,omitempty"`
	KubeName      string           `json:"kube_name,omitempty"`
	Key           string           `json:"key,omitempty"`
	Action        string           `json:"action"`
	Changes       []AuditChangeOut `json:"changes"`
	BatchId       *string          `json:"batch_id,omitempty"`
}

type AuditListOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []AuditOut    `json:"results"`
}

func auditChangeToOut(v auditModel.Change, _ int) AuditChangeOut {
	return AuditChangeOut{
		Field:     v.Field,
		Old:       v.Old,
		New:       v.New,
		OldHash:   v.OldHash,
		NewHash:   v.NewHash,
		OldSize:   v.OldSize,
		NewSize:   v.NewSize,
		Truncated: v.Truncated,
	}
}

func auditToOut(v *auditModel.Main, _ int) AuditOut {
	return AuditOut{
		Id:            v.Id,
		CreatedAt:     timeOut(v.CreatedAt),
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     v.ActorName,
		Source:        v.Source,
		EntityType:    v.EntityType,
		EntityId:      v.EntityId,
		AppId:         v.AppId,
		Namespace:     v.Namespace,
		AppSlug:       v.AppSlug,
		KubeKind:      v.KubeKind,
		KubeName:      v.KubeName,
		Key:           v.Key,
		Action:        v.Action,
		Changes:       lo.Map(v.Changes, auditChangeToOut),
		BatchId:       v.BatchId,
	}
}

func (s *sessionServer) auditList(ctx context.Context, req *mcpsdk.CallToolRequest, in AuditListIn) (*mcpsdk.CallToolResult, AuditListOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, AuditListOut{}, err
	}

	createdAtGte, err := parseTimePtr("created_at_gte", in.CreatedAtGte)
	if err != nil {
		return nil, AuditListOut{}, err
	}
	createdAtLt, err := parseTimePtr("created_at_lt", in.CreatedAtLt)
	if err != nil {
		return nil, AuditListOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.auditUsecase.List(ctx, &auditModel.ListReq{
		ListParams:   lp,
		AppId:        optStr(in.AppId),
		Namespace:    optStr(in.Namespace),
		AppSlug:      optStr(in.AppSlug),
		KubeName:     optStr(in.KubeName),
		EntityType:   optStr(in.EntityType),
		EntityId:     optStr(in.EntityId),
		Action:       optStr(in.Action),
		ActorUsrId:   in.ActorUsrId,
		Key:          optStr(in.Key),
		BatchId:      optStr(in.BatchId),
		CreatedAtGte: createdAtGte,
		CreatedAtLt:  createdAtLt,
	})
	if err != nil {
		return nil, AuditListOut{}, s.toolErr(err)
	}

	return nil, AuditListOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, auditToOut),
	}, nil
}

// ── sync_run_list ───────────────────────────────────────

type SyncRunListIn struct {
	PageIn
	Id           string `json:"id,omitempty" jsonschema:"id запуска — вернуть один запуск с затронутыми объектами"`
	AppId        string `json:"app_id,omitempty" jsonschema:"фильтр по app"`
	Namespace    string `json:"namespace,omitempty" jsonschema:"запуски, затронувшие объекты этого namespace"`
	Status       string `json:"status,omitempty" jsonschema:"running | ok | partial | error"`
	StartedAtGte string `json:"started_at_gte,omitempty" jsonschema:"нижняя граница времени старта (RFC3339)"`
	StartedAtLt  string `json:"started_at_lt,omitempty" jsonschema:"верхняя граница времени старта (RFC3339)"`
}

type SyncRunObjectOut struct {
	Namespace   string   `json:"namespace"`
	KubeKind    string   `json:"kube_kind"`
	KubeName    string   `json:"kube_name"`
	Op          string   `json:"op"`
	Error       string   `json:"error,omitempty"`
	ContentHash string   `json:"content_hash,omitempty"`
	ChangedKeys []string `json:"changed_keys"`
}

type SyncRunOut struct {
	Id            string             `json:"id"`
	StartedAt     string             `json:"started_at"`
	FinishedAt    string             `json:"finished_at,omitempty"`
	Status        string             `json:"status"`
	Error         string             `json:"error,omitempty"`
	ActorUsrId    *int64             `json:"actor_usr_id,omitempty"`
	ActorApiKeyId *string            `json:"actor_api_key_id,omitempty"`
	ActorName     string             `json:"actor_name"`
	Source        string             `json:"source"`
	AppId         *string            `json:"app_id,omitempty" jsonschema:"null — sync всех доступных app"`
	DurationMs    int64              `json:"duration_ms"`
	Objects       []SyncRunObjectOut `json:"objects,omitempty" jsonschema:"заполняется только при запросе по id"`
}

type SyncRunListOut struct {
	PaginationInfo PaginationOut `json:"pagination_info"`
	Results        []SyncRunOut  `json:"results"`
}

func syncRunObjectToOut(v *syncrunModel.Object, _ int) SyncRunObjectOut {
	changedKeys := v.ChangedKeys
	if changedKeys == nil {
		changedKeys = []string{}
	}
	return SyncRunObjectOut{
		Namespace:   v.Namespace,
		KubeKind:    v.KubeKind,
		KubeName:    v.KubeName,
		Op:          v.Op,
		Error:       v.Error,
		ContentHash: v.ContentHash,
		ChangedKeys: changedKeys,
	}
}

func syncRunToOut(v *syncrunModel.Main, _ int) SyncRunOut {
	return SyncRunOut{
		Id:            v.Id,
		StartedAt:     timeOut(v.StartedAt),
		FinishedAt:    optTimeOut(v.FinishedAt),
		Status:        v.Status,
		Error:         v.Error,
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     v.ActorName,
		Source:        v.Source,
		AppId:         v.AppId,
		DurationMs:    v.DurationMs,
		Objects:       lo.Map(v.Objects, syncRunObjectToOut),
	}
}

func (s *sessionServer) syncRunList(ctx context.Context, req *mcpsdk.CallToolRequest, in SyncRunListIn) (*mcpsdk.CallToolResult, SyncRunListOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, SyncRunListOut{}, err
	}

	// по id — один запуск с объектами
	if in.Id != "" {
		run, err := s.h.syncrunUsecase.Get(ctx, in.Id)
		if err != nil {
			return nil, SyncRunListOut{}, s.toolErr(err)
		}
		return nil, SyncRunListOut{
			PaginationInfo: PaginationOut{PageSize: 1, TotalCount: 1},
			Results:        []SyncRunOut{syncRunToOut(run, 0)},
		}, nil
	}

	startedAtGte, err := parseTimePtr("started_at_gte", in.StartedAtGte)
	if err != nil {
		return nil, SyncRunListOut{}, err
	}
	startedAtLt, err := parseTimePtr("started_at_lt", in.StartedAtLt)
	if err != nil {
		return nil, SyncRunListOut{}, err
	}

	lp := in.listParams()
	items, tCount, err := s.h.syncrunUsecase.List(ctx, &syncrunModel.ListReq{
		ListParams:   lp,
		AppId:        optStr(in.AppId),
		Namespace:    optStr(in.Namespace),
		Status:       optStr(in.Status),
		StartedAtGte: startedAtGte,
		StartedAtLt:  startedAtLt,
	})
	if err != nil {
		return nil, SyncRunListOut{}, s.toolErr(err)
	}

	return nil, SyncRunListOut{
		PaginationInfo: paginationOut(lp, tCount),
		Results:        lo.Map(items, syncRunToOut),
	}, nil
}
