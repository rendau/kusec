package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/app"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type App struct {
	proto.UnsafeAppServer
	usecase *usecase.Usecase
}

func NewApp(uc *usecase.Usecase) *App {
	return &App{usecase: uc}
}

func (h *App) List(ctx context.Context, req *proto.AppListReq) (*proto.AppListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeAppListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.AppListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeAppMain),
	}, nil
}

func (h *App) Get(ctx context.Context, req *proto.AppGetReq) (*proto.AppMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeAppMain(item, 0), nil
}

func (h *App) Create(ctx context.Context, req *proto.AppCreateReq) (*proto.AppCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeAppCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &proto.AppCreateRep{Id: newId}, nil
}

func (h *App) Update(ctx context.Context, req *proto.AppUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeAppUpdateReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *App) Delete(ctx context.Context, req *proto.AppGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
