package kube

import (
	"context"

	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	kubeService "github.com/mechta-market/kusec/internal/service/kube"
)

type KubeServiceI interface {
	SyncSecrets(ctx context.Context, appIds []string) (*kubeService.SyncResult, error)
	ListNamespaces(ctx context.Context) ([]string, bool, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
