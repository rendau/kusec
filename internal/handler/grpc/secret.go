package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/secret"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type Secret struct {
	proto.UnsafeSecretServer
	usecase *usecase.Usecase
}

func NewSecret(uc *usecase.Usecase) *Secret {
	return &Secret{usecase: uc}
}

func (h *Secret) List(ctx context.Context, req *proto.SecretListReq) (*proto.SecretListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeSecretListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.SecretListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeSecretMain),
	}, nil
}

func (h *Secret) Get(ctx context.Context, req *proto.SecretGetReq) (*proto.SecretMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeSecretMain(item, 0), nil
}

func (h *Secret) Create(ctx context.Context, req *proto.SecretCreateReq) (*proto.SecretCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeSecretCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &proto.SecretCreateRep{Id: newId}, nil
}

func (h *Secret) Update(ctx context.Context, req *proto.SecretUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeSecretUpdateReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Secret) Delete(ctx context.Context, req *proto.SecretGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
