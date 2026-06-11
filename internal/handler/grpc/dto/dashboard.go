package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	usecaseModel "github.com/mechta-market/kusec/internal/usecase/dashboard"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// usecase → proto

func EncodeDashboardCount(v usecaseModel.Count) *proto.DashboardCountSt {
	return &proto.DashboardCountSt{
		Total:  v.Total,
		Active: v.Active,
	}
}

func EncodeDashboardRecentSecret(v *usecaseModel.RecentSecret, _ int) *proto.DashboardRecentSecretSt {
	if v == nil {
		return nil
	}
	return &proto.DashboardRecentSecretSt{
		Id:          v.Id,
		AppId:       v.AppId,
		AppName:     v.AppName,
		SlugName:    v.SlugName,
		Description: v.Description,
		Active:      v.Active,
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		ItemCount:   v.ItemCount,
	}
}
