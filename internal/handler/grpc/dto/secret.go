package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	domainModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeSecretMain(v *domainModel.Main, _ int) *proto.SecretMain {
	if v == nil {
		return nil
	}
	return &proto.SecretMain{
		Id:             v.Id,
		CreatedAt:      timestamppb.New(v.CreatedAt),
		UpdatedAt:      timestamppb.New(v.UpdatedAt),
		AppId:          v.AppId,
		Active:         v.Active,
		SlugName:       v.SlugName,
		Description:    v.Description,
		KubeSecretName: v.KubeSecretName,
		KubeType:       v.KubeType,
	}
}

// proto → domain

func DecodeSecretListReq(v *proto.SecretListReq) *domainModel.ListReq {
	if v == nil {
		return nil
	}
	return &domainModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		AppId:      v.AppId,
		Active:     v.Active,
		Search:     v.Search,
	}
}

func DecodeSecretCreateReq(v *proto.SecretCreateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		AppId:       &v.AppId,
		Active:      v.Active,
		SlugName:    &v.SlugName,
		Description: &v.Description,
		KubeType:    &v.KubeType,
	}
}

func DecodeSecretUpdateReq(v *proto.SecretUpdateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		AppId:       v.AppId,
		Active:      v.Active,
		SlugName:    v.SlugName,
		Description: v.Description,
		KubeType:    v.KubeType,
	}
}
