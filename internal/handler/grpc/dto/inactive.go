package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	usecaseModel "github.com/rendau/kusec/internal/usecase/inactive"
	proto "github.com/rendau/kusec/pkg/proto/kusec_v1"
)

// usecase → proto

func EncodeInactiveElement(v *usecaseModel.Element, _ int) *proto.InactiveElementSt {
	if v == nil {
		return nil
	}
	return &proto.InactiveElementSt{
		Kind:        v.Kind,
		Id:          v.Id,
		AppId:       v.AppId,
		AppName:     v.AppName,
		ParentId:    v.ParentId,
		ParentName:  v.ParentName,
		Name:        v.Name,
		Description: v.Description,
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		ChildCount:  v.ChildCount,
	}
}

// proto → usecase

func DecodeInactiveRef(v *proto.InactiveRefSt, _ int) usecaseModel.Ref {
	return usecaseModel.Ref{Kind: v.GetKind(), Id: v.GetId()}
}
