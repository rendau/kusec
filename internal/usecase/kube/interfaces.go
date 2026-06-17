package kube

import (
	"context"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	kubeService "github.com/mechta-market/kusec/internal/service/kube"
)

type KubeServiceI interface {
	SyncSecrets(ctx context.Context, appIds []string) (*kubeService.SyncResult, error)
	SyncConfigMaps(ctx context.Context, appIds []string) (*kubeService.SyncResult, error)
	Sync(ctx context.Context, appIds []string) (*kubeService.SyncResult, *kubeService.SyncResult, error)
	ListNamespaces(ctx context.Context) ([]string, bool, error)
	ListClusterSecrets(ctx context.Context, namespace string) ([]*kubeService.ClusterSecret, bool, error)
	ImportSecret(ctx context.Context, appId string, ref kubeService.ImportRef, secretSlug string) (*kubeService.ImportResult, error)
	GetClusterSecret(ctx context.Context, namespace, name string) (*kubeService.ClusterResource, bool, bool, error)
	GetClusterConfigMap(ctx context.Context, namespace, name string) (*kubeService.ClusterResource, bool, bool, error)
}

type AppServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*appModel.Main, bool, error)
}

type SecretServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*secretModel.Main, bool, error)
}

type ConfigMapServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*configmapModel.Main, bool, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
