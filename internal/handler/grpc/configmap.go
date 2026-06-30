package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/configmap"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type ConfigMap struct {
	proto.UnsafeConfigMapServer
	usecase *usecase.Usecase
}

func NewConfigMap(uc *usecase.Usecase) *ConfigMap {
	return &ConfigMap{usecase: uc}
}

func (h *ConfigMap) List(ctx context.Context, req *proto.ConfigMapListReq) (*proto.ConfigMapListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeConfigMapListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.ConfigMapListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeConfigMapMain),
	}, nil
}

func (h *ConfigMap) Get(ctx context.Context, req *proto.ConfigMapGetReq) (*proto.ConfigMapMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeConfigMapMain(item, 0), nil
}

func (h *ConfigMap) Create(ctx context.Context, req *proto.ConfigMapCreateReq) (*proto.ConfigMapCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeConfigMapCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &proto.ConfigMapCreateRep{Id: newId}, nil
}

func (h *ConfigMap) Update(ctx context.Context, req *proto.ConfigMapUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeConfigMapUpdateReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *ConfigMap) Delete(ctx context.Context, req *proto.ConfigMapGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
