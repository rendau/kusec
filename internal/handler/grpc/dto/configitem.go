package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	domainModel "github.com/rendau/kusec/internal/domain/configitem/model"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeConfigItemMain(v *domainModel.Main, _ int) *proto.ConfigItemMain {
	if v == nil {
		return nil
	}
	return &proto.ConfigItemMain{
		Id:          v.Id,
		CreatedAt:   timestamppb.New(v.CreatedAt),
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		ConfigmapId: v.ConfigMapId,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: v.Description,
	}
}

// proto → domain

func DecodeConfigItemListReq(v *proto.ConfigItemListReq) *domainModel.ListReq {
	if v == nil {
		return nil
	}
	return &domainModel.ListReq{
		ListParams:   DecodeListParams(v.ListParams),
		ConfigMapId:  v.ConfigmapId,
		ConfigMapIds: v.ConfigmapIds,
		Active:       v.Active,
		Search:       v.Search,
	}
}

func DecodeConfigItemCreateReq(v *proto.ConfigItemCreateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		ConfigMapId: &v.ConfigmapId,
		Active:      v.Active,
		Key:         &v.Key,
		Value:       &v.Value,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: &v.Description,
	}
}

func DecodeConfigItemUpdateReq(v *proto.ConfigItemUpdateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		ConfigMapId: v.ConfigmapId,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: v.Description,
	}
}
