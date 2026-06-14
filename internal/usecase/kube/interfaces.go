package kube

import (
	"context"

	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	kubeService "github.com/mechta-market/kusec/internal/service/kube"
)

type KubeServiceI interface {
	SyncSecrets(ctx context.Context, appIds []string) (*kubeService.SyncResult, error)
	SyncConfigMaps(ctx context.Context, appIds []string) (*kubeService.SyncResult, error)
	Sync(ctx context.Context, appIds []string) (*kubeService.SyncResult, *kubeService.SyncResult, error)
	ListNamespaces(ctx context.Context) ([]string, bool, error)
	ListClusterSecrets(ctx context.Context, namespace string) ([]*kubeService.ClusterSecret, bool, error)
	ImportSecrets(ctx context.Context, appId string, refs []kubeService.ImportRef) (*kubeService.ImportResult, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
