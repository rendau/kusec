package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	kubeService "github.com/rendau/kusec/internal/service/kube"
)

// ── Регистрация ─────────────────────────────────────────

func (s *sessionServer) registerKubeTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "sync",
		Description: "Синхронизировать секреты и конфигмапы из kusec в Kubernetes-кластер (создать/обновить/удалить управляемые k8s-объекты). По умолчанию синхронизируется только текущий app (use_app); all_apps=true — все доступные app. Работает, только когда kusec запущен внутри кластера.",
	}, s.sync)
}

// ── Sync ────────────────────────────────────────────────

type SyncIn struct {
	AllApps bool `json:"all_apps,omitempty" jsonschema:"true — синхронизировать все доступные app; по умолчанию только текущий app"`
}

// SyncResultOut — итог синхронизации одного вида объектов; списки в формате "namespace/name".
type SyncResultOut struct {
	Created   []string `json:"created"`
	Updated   []string `json:"updated"`
	Deleted   []string `json:"deleted"`
	Unchanged int64    `json:"unchanged"`
	Errors    []string `json:"errors,omitempty" jsonschema:"ошибки по отдельным объектам: sync не прерывается, проблемные объекты пропускаются"`
}

type SyncOut struct {
	Secrets    SyncResultOut `json:"secrets"`
	Configmaps SyncResultOut `json:"configmaps"`
}

func (s *sessionServer) sync(ctx context.Context, req *mcpsdk.CallToolRequest, in SyncIn) (*mcpsdk.CallToolResult, SyncOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, SyncOut{}, err
	}

	var appId *string
	if !in.AllApps {
		app, cerr := s.currentApp()
		if cerr != nil {
			return nil, SyncOut{}, s.toolErr(cerr)
		}
		appId = &app.Id
	}

	secrets, configMaps, err := s.h.kubeUsecase.Sync(ctx, appId)
	if err != nil {
		return nil, SyncOut{}, s.toolErr(err)
	}

	return nil, SyncOut{
		Secrets:    s.syncResultOut(secrets),
		Configmaps: s.syncResultOut(configMaps),
	}, nil
}

// syncResultOut конвертирует итог синхронизации, вычищая значения секретов из
// текстов ошибок по отдельным объектам.
func (s *sessionServer) syncResultOut(v *kubeService.SyncResult) SyncResultOut {
	if v == nil {
		return SyncResultOut{Created: []string{}, Updated: []string{}, Deleted: []string{}}
	}

	return SyncResultOut{
		Created:   v.Created,
		Updated:   v.Updated,
		Deleted:   v.Deleted,
		Unchanged: v.Unchanged,
		Errors:    lo.Map(v.Errors, func(e string, _ int) string { return s.vault.scrub(e) }),
	}
}
