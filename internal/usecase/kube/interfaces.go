package kube

import (
	"context"

	kubeService "github.com/mechta-market/kusec/internal/service/kube"
)

type KubeServiceI interface {
	SyncSecrets(ctx context.Context) (*kubeService.SyncResult, error)
	ListNamespaces(ctx context.Context) ([]string, bool, error)
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
