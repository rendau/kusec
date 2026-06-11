package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mechta-market/kusec/internal/handler/grpc/dto"
	usecase "github.com/mechta-market/kusec/internal/usecase/usr"
	"github.com/mechta-market/kusec/pkg/proto/common"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

type Usr struct {
	proto.UnsafeUsrServer
	usecase *usecase.Usecase
}

func NewUsr(uc *usecase.Usecase) *Usr {
	return &Usr{usecase: uc}
}

func (h *Usr) List(ctx context.Context, req *proto.UsrListReq) (*proto.UsrListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &common.ListParamsSt{}
	}

	items, tCount, err := h.usecase.List(ctx, dto.DecodeUsrListReq(req))
	if err != nil {
		return nil, err
	}

	return &proto.UsrListRep{
		PaginationInfo: &common.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeUsrMain),
	}, nil
}

func (h *Usr) Get(ctx context.Context, req *proto.UsrGetReq) (*proto.UsrMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeUsrMain(item, 0), nil
}

func (h *Usr) Create(ctx context.Context, req *proto.UsrCreateReq) (*proto.UsrCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeUsrCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &proto.UsrCreateRep{Id: newId}, nil
}

func (h *Usr) Update(ctx context.Context, req *proto.UsrUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeUsrUpdateReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Usr) Delete(ctx context.Context, req *proto.UsrGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Usr) Login(ctx context.Context, req *proto.UsrLoginReq) (*proto.UsrLoginRep, error) {
	accessToken, refreshToken, err := h.usecase.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return &proto.UsrLoginRep{Jwt: accessToken, RefreshToken: refreshToken}, nil
}

func (h *Usr) RefreshToken(ctx context.Context, req *proto.UsrRefreshTokenReq) (*proto.UsrLoginRep, error) {
	accessToken, refreshToken, err := h.usecase.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &proto.UsrLoginRep{Jwt: accessToken, RefreshToken: refreshToken}, nil
}

func (h *Usr) BootstrapStatus(ctx context.Context, _ *emptypb.Empty) (*proto.UsrBootstrapStatusRep, error) {
	canCreateFirstAdmin, err := h.usecase.BootstrapStatus(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.UsrBootstrapStatusRep{CanCreateFirstAdmin: canCreateFirstAdmin}, nil
}

func (h *Usr) GetProfile(ctx context.Context, _ *emptypb.Empty) (*proto.UsrMain, error) {
	item, err := h.usecase.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeUsrMain(item, 0), nil
}

func (h *Usr) UpdateProfile(ctx context.Context, req *proto.UsrUpdateProfileReq) (*emptypb.Empty, error) {
	if err := h.usecase.UpdateProfile(ctx, dto.DecodeUsrUpdateProfileReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
