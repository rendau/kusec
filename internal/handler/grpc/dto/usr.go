package dto

import (
	domainModel "github.com/rendau/kusec/internal/domain/usr/model"
	usecase "github.com/rendau/kusec/internal/usecase/usr"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// domain → proto

func EncodeUsrMain(v *domainModel.Main, _ int) *proto.UsrMain {
	if v == nil {
		return nil
	}
	return &proto.UsrMain{
		Id:          v.Id,
		Active:      v.Active,
		IsAdmin:     v.IsAdmin,
		Name:        v.Name,
		Username:    v.Username,
		AppIds:      v.AppIds,
		TotpEnabled: v.TotpEnabled,
	}
}

func EncodeUsrLoginResult(v *usecase.LoginResult) *proto.UsrLoginRep {
	if v == nil {
		return &proto.UsrLoginRep{}
	}
	return &proto.UsrLoginRep{
		Jwt:               v.Jwt,
		RefreshToken:      v.RefreshToken,
		TotpRequired:      v.TotpRequired,
		TotpSetupRequired: v.TotpSetupRequired,
		SetupToken:        v.SetupToken,
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
		AppIds:   v.AppIds,
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
		AppIds:   v.AppIds,
	}
}

func DecodeUsrUpdateProfileReq(v *proto.UsrUpdateProfileReq) *usecase.UpdateProfileReq {
	if v == nil {
		return nil
	}
	return &usecase.UpdateProfileReq{
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}
