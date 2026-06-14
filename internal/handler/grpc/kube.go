package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mechta-market/kusec/internal/handler/grpc/dto"
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
		Secrets:   lo.Map(secrets, dto.EncodeKubeClusterSecret),
	}, nil
}

func (h *Kube) ImportSecrets(ctx context.Context, req *proto.KubeImportSecretsReq) (*proto.KubeImportSecretsRep, error) {
	var appId string
	var refs []kubeService.ImportRef
	if req != nil {
		appId = req.AppId
		refs = lo.Map(req.Secrets, dto.DecodeKubeImportRef)
	}

	result, err := h.usecase.ImportSecrets(ctx, appId, refs)
	if err != nil {
		return nil, err
	}

	return dto.EncodeKubeImportResult(result), nil
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

	return dto.EncodeKubeSyncSecretsRep(result), nil
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

	return dto.EncodeKubeSyncConfigMapsRep(result), nil
}

func (h *Kube) Sync(ctx context.Context, req *proto.KubeSyncReq) (*proto.KubeSyncRep, error) {
	var appId *string
	if req != nil && req.AppId != "" {
		appId = &req.AppId
	}

	secrets, configMaps, err := h.usecase.Sync(ctx, appId)
	if err != nil {
		return nil, err
	}

	return &proto.KubeSyncRep{
		Secrets:    dto.EncodeKubeSyncSecretsRep(secrets),
		Configmaps: dto.EncodeKubeSyncConfigMapsRep(configMaps),
	}, nil
}
