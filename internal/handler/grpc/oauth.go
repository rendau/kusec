package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/oauth"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type OAuth struct {
	proto.UnsafeOAuthServer
	usecase *usecase.Usecase
}

func NewOAuth(uc *usecase.Usecase) *OAuth {
	return &OAuth{usecase: uc}
}

func (h *OAuth) DescribeAuthorization(ctx context.Context, req *proto.OAuthAuthorizeReq) (*proto.OAuthAuthorizeInfoRep, error) {
	info, err := h.usecase.DescribeAuthorization(ctx, dto.DecodeOAuthAuthorizeReq(req))
	if err != nil {
		return nil, err
	}
	return dto.EncodeOAuthAuthorizeInfo(info), nil
}

func (h *OAuth) Authorize(ctx context.Context, req *proto.OAuthAuthorizeReq) (*proto.OAuthAuthorizeRep, error) {
	redirectUrl, err := h.usecase.Authorize(ctx, dto.DecodeOAuthAuthorizeReq(req), req.Approve)
	if err != nil {
		return nil, err
	}
	return &proto.OAuthAuthorizeRep{RedirectUrl: redirectUrl}, nil
}

func (h *OAuth) ListGrants(ctx context.Context, _ *emptypb.Empty) (*proto.OAuthGrantListRep, error) {
	items, err := h.usecase.ListGrants(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.OAuthGrantListRep{Results: lo.Map(items, dto.EncodeOAuthGrantMain)}, nil
}

func (h *OAuth) DeleteGrant(ctx context.Context, req *proto.OAuthGrantDeleteReq) (*emptypb.Empty, error) {
	if err := h.usecase.DeleteGrant(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
