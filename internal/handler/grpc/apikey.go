package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mechta-market/kusec/internal/handler/grpc/dto"
	usecase "github.com/mechta-market/kusec/internal/usecase/apikey"
	"github.com/mechta-market/kusec/pkg/proto/common"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

type ApiKey struct {
	proto.UnsafeApiKeyServer
	usecase *usecase.Usecase
}

func NewApiKey(uc *usecase.Usecase) *ApiKey {
	return &ApiKey{usecase: uc}
}

func (h *ApiKey) List(ctx context.Context, req *proto.ApiKeyListReq) (*proto.ApiKeyListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &common.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeApiKeyListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.ApiKeyListRep{
		PaginationInfo: &common.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeApiKeyMain),
	}, nil
}

func (h *ApiKey) Create(ctx context.Context, req *proto.ApiKeyCreateReq) (*proto.ApiKeyCreateRep, error) {
	newId, key, err := h.usecase.Create(ctx, req.Name, req.UsrId, req.McpOnly)
	if err != nil {
		return nil, err
	}
	return &proto.ApiKeyCreateRep{Id: newId, Key: key}, nil
}

func (h *ApiKey) Update(ctx context.Context, req *proto.ApiKeyUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, req.Active, req.Name, req.McpOnly); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *ApiKey) Delete(ctx context.Context, req *proto.ApiKeyGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
