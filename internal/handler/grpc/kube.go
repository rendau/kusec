package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

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

func (h *Kube) SyncSecrets(ctx context.Context, _ *emptypb.Empty) (*proto.KubeSyncSecretsRep, error) {
	result, err := h.usecase.SyncSecrets(ctx)
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
