package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/kusec/internal/handler/grpc/dto"
	usecase "github.com/rendau/kusec/internal/usecase/dashboard"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

type Dashboard struct {
	proto.UnsafeDashboardServer
	usecase *usecase.Usecase
}

func NewDashboard(uc *usecase.Usecase) *Dashboard {
	return &Dashboard{usecase: uc}
}

func (h *Dashboard) Get(ctx context.Context, _ *emptypb.Empty) (*proto.DashboardRep, error) {
	summary, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}

	return &proto.DashboardRep{
		App:           dto.EncodeDashboardCount(summary.App),
		Secret:        dto.EncodeDashboardCount(summary.Secret),
		Item:          dto.EncodeDashboardCount(summary.Item),
		Configmap:     dto.EncodeDashboardCount(summary.ConfigMap),
		ConfigItem:    dto.EncodeDashboardCount(summary.ConfigItem),
		Usr:           dto.EncodeDashboardCount(summary.Usr),
		RecentSecrets: lo.Map(summary.RecentSecrets, dto.EncodeDashboardRecentSecret),
	}, nil
}
