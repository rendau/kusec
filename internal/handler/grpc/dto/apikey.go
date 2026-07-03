package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	domainModel "github.com/mechta-market/kusec/internal/domain/apikey/model"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeApiKeyMain(v *domainModel.Main, _ int) *proto.ApiKeyMain {
	if v == nil {
		return nil
	}

	result := &proto.ApiKeyMain{
		Id:        v.Id,
		CreatedAt: timestamppb.New(v.CreatedAt),
		UpdatedAt: timestamppb.New(v.UpdatedAt),
		UsrId:     v.UsrId,
		Active:    v.Active,
		Name:      v.Name,
		KeyPrefix: v.KeyPrefix,
	}
	if v.LastUsedAt != nil {
		result.LastUsedAt = timestamppb.New(*v.LastUsedAt)
	}

	return result
}

// proto → domain

func DecodeApiKeyListReq(v *proto.ApiKeyListReq) *domainModel.ListReq {
	if v == nil {
		return nil
	}
	return &domainModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		UsrId:      v.UsrId,
		Active:     v.Active,
	}
}
