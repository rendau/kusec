package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	domainModel "github.com/mechta-market/kusec/internal/domain/item/model"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeItemMain(v *domainModel.Main, _ int) *proto.ItemMain {
	if v == nil {
		return nil
	}
	return &proto.ItemMain{
		Id:          v.Id,
		CreatedAt:   timestamppb.New(v.CreatedAt),
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		SecretId:    v.SecretId,
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

func DecodeItemListReq(v *proto.ItemListReq) *domainModel.ListReq {
	if v == nil {
		return nil
	}
	return &domainModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		SecretId:   v.SecretId,
		SecretIds:  v.SecretIds,
		Active:     v.Active,
		Search:     v.Search,
	}
}

func DecodeItemCreateReq(v *proto.ItemCreateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		SecretId:    &v.SecretId,
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

func DecodeItemUpdateReq(v *proto.ItemUpdateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		SecretId:    v.SecretId,
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
