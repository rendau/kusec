package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/constant"
	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/apikey"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
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
		req.ListParams = &proto.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeApiKeyListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.ApiKeyListRep{
		PaginationInfo: &proto.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeApiKeyMain),
	}, nil
}

func (h *ApiKey) Create(ctx context.Context, req *proto.ApiKeyCreateReq) (*proto.ApiKeyCreateRep, error) {
	scope := ""
	if req.Scope != nil {
		scope = *req.Scope
	} else if req.McpOnly { //nolint:staticcheck // совместимость со старым полем
		scope = constant.ApiKeyScopeMcpOnly
	}

	newId, key, err := h.usecase.Create(ctx, req.Name, req.UsrId, scope)
	if err != nil {
		return nil, err
	}
	return &proto.ApiKeyCreateRep{Id: newId, Key: key}, nil
}

func (h *ApiKey) Update(ctx context.Context, req *proto.ApiKeyUpdateReq) (*emptypb.Empty, error) {
	scope := req.Scope
	if scope == nil && req.McpOnly != nil { //nolint:staticcheck // совместимость со старым полем
		if *req.McpOnly { //nolint:staticcheck
			scope = new(constant.ApiKeyScopeMcpOnly)
		} else {
			scope = new(constant.ApiKeyScopeFull)
		}
	}

	if err := h.usecase.Update(ctx, req.Id, req.Active, req.Name, scope); err != nil {
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
