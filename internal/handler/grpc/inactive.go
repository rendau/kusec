package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/inactive"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type Inactive struct {
	proto.UnsafeInactiveServer
	usecase *usecase.Usecase
}

func NewInactive(uc *usecase.Usecase) *Inactive {
	return &Inactive{usecase: uc}
}

func (h *Inactive) List(ctx context.Context, req *proto.InactiveListReq) (*proto.InactiveListRep, error) {
	elements, err := h.usecase.List(ctx, req.AppId)
	if err != nil {
		return nil, err
	}

	return &proto.InactiveListRep{
		Results: lo.Map(elements, dto.EncodeInactiveElement),
	}, nil
}

func (h *Inactive) Delete(ctx context.Context, req *proto.InactiveDeleteReq) (*emptypb.Empty, error) {
	err := h.usecase.Delete(ctx, lo.Map(req.Elements, dto.DecodeInactiveRef))
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
