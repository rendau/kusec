package grpc

import (
	"context"

	"github.com/samber/lo"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/audit"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type Audit struct {
	proto.UnsafeAuditServer
	usecase *usecase.Usecase
}

func NewAudit(uc *usecase.Usecase) *Audit {
	return &Audit{usecase: uc}
}

func (h *Audit) List(ctx context.Context, req *proto.AuditListReq) (*proto.AuditListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeAuditListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.AuditListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeAuditMain),
	}, nil
}
