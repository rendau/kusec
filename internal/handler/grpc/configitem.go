package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/configitem"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type ConfigItem struct {
	proto.UnsafeConfigItemServer
	usecase *usecase.Usecase
}

func NewConfigItem(uc *usecase.Usecase) *ConfigItem {
	return &ConfigItem{usecase: uc}
}

func (h *ConfigItem) List(ctx context.Context, req *proto.ConfigItemListReq) (*proto.ConfigItemListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeConfigItemListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.ConfigItemListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeConfigItemMain),
	}, nil
}

func (h *ConfigItem) Get(ctx context.Context, req *proto.ConfigItemGetReq) (*proto.ConfigItemMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeConfigItemMain(item, 0), nil
}

func (h *ConfigItem) Create(ctx context.Context, req *proto.ConfigItemCreateReq) (*proto.ConfigItemCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeConfigItemCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &proto.ConfigItemCreateRep{Id: newId}, nil
}

func (h *ConfigItem) Update(ctx context.Context, req *proto.ConfigItemUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeConfigItemUpdateReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *ConfigItem) Delete(ctx context.Context, req *proto.ConfigItemGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
