package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	domainModel "github.com/rendau/kusec/internal/domain/configmap/model"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeConfigMapMain(v *domainModel.Main, _ int) *proto.ConfigMapMain {
	if v == nil {
		return nil
	}
	return &proto.ConfigMapMain{
		Id:                v.Id,
		CreatedAt:         timestamppb.New(v.CreatedAt),
		UpdatedAt:         timestamppb.New(v.UpdatedAt),
		AppId:             v.AppId,
		Active:            v.Active,
		SlugName:          v.SlugName,
		Description:       v.Description,
		KubeConfigmapName: v.KubeConfigMapName,
		ExactSlug:         v.ExactSlug,
	}
}

// proto → domain

func DecodeConfigMapListReq(v *proto.ConfigMapListReq) *domainModel.ListReq {
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

func DecodeConfigMapCreateReq(v *proto.ConfigMapCreateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		AppId:       &v.AppId,
		Active:      v.Active,
		SlugName:    &v.SlugName,
		Description: &v.Description,
		ExactSlug:   &v.ExactSlug,
	}
}

func DecodeConfigMapUpdateReq(v *proto.ConfigMapUpdateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		AppId:       v.AppId,
		Active:      v.Active,
		SlugName:    v.SlugName,
		Description: v.Description,
		ExactSlug:   v.ExactSlug,
	}
}
