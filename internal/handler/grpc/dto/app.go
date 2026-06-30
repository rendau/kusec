package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	domainModel "github.com/rendau/kusec/internal/domain/app/model"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeAppMain(v *domainModel.Main, _ int) *proto.AppMain {
	if v == nil {
		return nil
	}
	return &proto.AppMain{
		Id:          v.Id,
		CreatedAt:   timestamppb.New(v.CreatedAt),
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		Active:      v.Active,
		Namespace:   v.Namespace,
		Name:        v.Name,
		SlugName:    v.SlugName,
		Description: v.Description,
	}
}

// proto → domain

func DecodeAppListReq(v *proto.AppListReq) *domainModel.ListReq {
	if v == nil {
		return nil
	}
	return &domainModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Active:     v.Active,
		Namespace:  v.Namespace,
		Search:     v.Search,
	}
}

func DecodeAppCreateReq(v *proto.AppCreateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		Active:      v.Active,
		Namespace:   &v.Namespace,
		Name:        &v.Name,
		SlugName:    &v.SlugName,
		Description: &v.Description,
	}
}

func DecodeAppUpdateReq(v *proto.AppUpdateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		Active:      v.Active,
		Namespace:   v.Namespace,
		Name:        v.Name,
		SlugName:    v.SlugName,
		Description: v.Description,
	}
}
