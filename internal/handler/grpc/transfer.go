package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/mechta-market/kusec/internal/handler/grpc/dto"
	usecase "github.com/mechta-market/kusec/internal/usecase/transfer"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

type Transfer struct {
	proto.UnsafeTransferServer
	usecase *usecase.Usecase
}

func NewTransfer(uc *usecase.Usecase) *Transfer {
	return &Transfer{usecase: uc}
}

func (h *Transfer) Tree(ctx context.Context, _ *emptypb.Empty) (*proto.TransferTreeRep, error) {
	apps, err := h.usecase.Tree(ctx)
	if err != nil {
		return nil, err
	}
	return &proto.TransferTreeRep{Apps: lo.Map(apps, dto.EncodeTransferTreeApp)}, nil
}
