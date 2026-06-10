package dto

import (
	domainModel "github.com/mechta-market/kusec/internal/domain/usr/model"
	usecase "github.com/mechta-market/kusec/internal/usecase/usr"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeUsrMain(v *domainModel.Main, _ int) *proto.UsrMain {
	if v == nil {
		return nil
	}
	return &proto.UsrMain{
		Id:       v.Id,
		Active:   v.Active,
		IsAdmin:  v.IsAdmin,
		Name:     v.Name,
		Username: v.Username,
	}
}

// proto → domain

func DecodeUsrListReq(v *proto.UsrListReq) *domainModel.ListReq {
	if v == nil {
		return nil
	}
	return &domainModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Active:     v.Active,
		IsAdmin:    v.IsAdmin,
		Search:     v.Search,
	}
}

func DecodeUsrCreateReq(v *proto.UsrCreateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		Active:   v.Active,
		IsAdmin:  &v.IsAdmin,
		Name:     &v.Name,
		Username: &v.Username,
		Password: &v.Password,
	}
}

func DecodeUsrUpdateReq(v *proto.UsrUpdateReq) *domainModel.Edit {
	if v == nil {
		return nil
	}
	return &domainModel.Edit{
		Active:   v.Active,
		IsAdmin:  v.IsAdmin,
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}

func DecodeUsrUpdateProfileReq(v *proto.UsrUpdateProfileReq) *usecase.UpdateProfileReq {
	if v == nil {
		return nil
	}
	return &usecase.UpdateProfileReq{
		Name:     v.Name,
		Password: v.Password,
	}
}
