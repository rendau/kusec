package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	kubeService "github.com/mechta-market/kusec/internal/service/kube"
	usecase "github.com/mechta-market/kusec/internal/usecase/kube"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

type Kube struct {
	proto.UnsafeKubeServer
	usecase *usecase.Usecase
}

func NewKube(uc *usecase.Usecase) *Kube {
	return &Kube{usecase: uc}
}

func (h *Kube) ListNamespaces(ctx context.Context, _ *emptypb.Empty) (*proto.KubeListNamespacesRep, error) {
	namespaces, inCluster, err := h.usecase.ListNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	return &proto.KubeListNamespacesRep{
		InCluster:  inCluster,
		Namespaces: namespaces,
	}, nil
}

func (h *Kube) ListClusterSecrets(ctx context.Context, req *proto.KubeListClusterSecretsReq) (*proto.KubeListClusterSecretsRep, error) {
	var namespace string
	if req != nil {
		namespace = req.Namespace
	}

	secrets, inCluster, err := h.usecase.ListClusterSecrets(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return &proto.KubeListClusterSecretsRep{
		InCluster: inCluster,
		Secrets: lo.Map(secrets, func(s *kubeService.ClusterSecret, _ int) *proto.KubeClusterSecretSt {
			return &proto.KubeClusterSecretSt{
				Namespace: s.Namespace,
				Name:      s.Name,
				Type:      s.Type,
				Keys:      s.Keys,
				Managed:   s.Managed,
			}
		}),
	}, nil
}

func (h *Kube) ImportSecrets(ctx context.Context, req *proto.KubeImportSecretsReq) (*proto.KubeImportSecretsRep, error) {
	var appId string
	var refs []kubeService.ImportRef
	if req != nil {
		appId = req.AppId
		refs = lo.Map(req.Secrets, func(s *proto.KubeImportSecretRefSt, _ int) kubeService.ImportRef {
			return kubeService.ImportRef{Namespace: s.Namespace, Name: s.Name}
		})
	}

	result, err := h.usecase.ImportSecrets(ctx, appId, refs)
	if err != nil {
		return nil, err
	}

	return &proto.KubeImportSecretsRep{
		Imported:       result.Imported,
		Skipped:        result.Skipped,
		Errors:         result.Errors,
		CreatedSecrets: result.CreatedSecrets,
		CreatedItems:   result.CreatedItems,
	}, nil
}

func (h *Kube) SyncSecrets(ctx context.Context, req *proto.KubeSyncSecretsReq) (*proto.KubeSyncSecretsRep, error) {
	var appId *string
	if req != nil && req.AppId != "" {
		appId = &req.AppId
	}

	result, err := h.usecase.SyncSecrets(ctx, appId)
	if err != nil {
		return nil, err
	}

	return &proto.KubeSyncSecretsRep{
		Created:   result.Created,
		Updated:   result.Updated,
		Deleted:   result.Deleted,
		Unchanged: result.Unchanged,
		Errors:    result.Errors,
	}, nil
}

func (h *Kube) SyncConfigMaps(ctx context.Context, req *proto.KubeSyncConfigMapsReq) (*proto.KubeSyncConfigMapsRep, error) {
	var appId *string
	if req != nil && req.AppId != "" {
		appId = &req.AppId
	}

	result, err := h.usecase.SyncConfigMaps(ctx, appId)
	if err != nil {
		return nil, err
	}

	return &proto.KubeSyncConfigMapsRep{
		Created:   result.Created,
		Updated:   result.Updated,
		Deleted:   result.Deleted,
		Unchanged: result.Unchanged,
		Errors:    result.Errors,
	}, nil
}
