package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mechta-market/kusec/internal/handler/grpc/dto"
	usecase "github.com/mechta-market/kusec/internal/usecase/item"
	"github.com/mechta-market/kusec/pkg/proto/common"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

type Item struct {
	proto.UnsafeItemServer
	usecase *usecase.Usecase
}

func NewItem(uc *usecase.Usecase) *Item {
	return &Item{usecase: uc}
}

func (h *Item) List(ctx context.Context, req *proto.ItemListReq) (*proto.ItemListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &common.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeItemListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.ItemListRep{
		PaginationInfo: &common.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeItemMain),
	}, nil
}

func (h *Item) Get(ctx context.Context, req *proto.ItemGetReq) (*proto.ItemMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeItemMain(item, 0), nil
}

func (h *Item) Create(ctx context.Context, req *proto.ItemCreateReq) (*proto.ItemCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeItemCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &proto.ItemCreateRep{Id: newId}, nil
}

func (h *Item) Update(ctx context.Context, req *proto.ItemUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeItemUpdateReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Item) Delete(ctx context.Context, req *proto.ItemGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
