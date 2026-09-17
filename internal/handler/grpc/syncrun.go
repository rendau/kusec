package grpc

import (
	"context"

	"github.com/samber/lo"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/syncrun"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type SyncRun struct {
	proto.UnsafeSyncRunServer
	usecase *usecase.Usecase
}

func NewSyncRun(uc *usecase.Usecase) *SyncRun {
	return &SyncRun{usecase: uc}
}

func (h *SyncRun) List(ctx context.Context, req *proto.SyncRunListReq) (*proto.SyncRunListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeSyncRunListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.SyncRunListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeSyncRunMain),
	}, nil
}

func (h *SyncRun) Get(ctx context.Context, req *proto.SyncRunGetReq) (*proto.SyncRunMain, error) {
	result, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeSyncRunMain(result, 0), nil
}
