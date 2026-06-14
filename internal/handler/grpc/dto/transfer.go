package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	usecaseModel "github.com/mechta-market/kusec/internal/usecase/transfer"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// usecase → proto

func EncodeTransferTreeApp(v *usecaseModel.TreeApp, _ int) *proto.TransferTreeAppSt {
	if v == nil {
		return nil
	}
	secrets := make([]*proto.TransferTreeSecretSt, 0, len(v.Secrets))
	for _, secret := range v.Secrets {
		items := make([]*proto.TransferTreeItemSt, 0, len(secret.Items))
		for _, item := range secret.Items {
			items = append(items, &proto.TransferTreeItemSt{
				Id:          item.Id,
				Key:         item.Key,
				ValueFormat: item.ValueFormat,
				Encoding:    item.Encoding,
				FileName:    item.FileName,
				ContentType: item.ContentType,
				Description: item.Description,
				Active:      item.Active,
				UpdatedAt:   timestamppb.New(item.UpdatedAt),
				ValueSize:   item.ValueSize,
			})
		}
		secrets = append(secrets, &proto.TransferTreeSecretSt{
			Id:          secret.Id,
			SlugName:    secret.SlugName,
			Description: secret.Description,
			Active:      secret.Active,
			KubeType:    secret.KubeType,
			UpdatedAt:   timestamppb.New(secret.UpdatedAt),
			Items:       items,
		})
	}
	configmaps := make([]*proto.TransferTreeConfigMapSt, 0, len(v.ConfigMaps))
	for _, configMap := range v.ConfigMaps {
		items := make([]*proto.TransferTreeConfigItemSt, 0, len(configMap.Items))
		for _, item := range configMap.Items {
			items = append(items, &proto.TransferTreeConfigItemSt{
				Id:          item.Id,
				Key:         item.Key,
				ValueFormat: item.ValueFormat,
				Encoding:    item.Encoding,
				FileName:    item.FileName,
				ContentType: item.ContentType,
				Description: item.Description,
				Active:      item.Active,
				UpdatedAt:   timestamppb.New(item.UpdatedAt),
				ValueSize:   item.ValueSize,
			})
		}
		configmaps = append(configmaps, &proto.TransferTreeConfigMapSt{
			Id:          configMap.Id,
			SlugName:    configMap.SlugName,
			Description: configMap.Description,
			Active:      configMap.Active,
			UpdatedAt:   timestamppb.New(configMap.UpdatedAt),
			Items:       items,
		})
	}
	return &proto.TransferTreeAppSt{
		Id:          v.Id,
		Namespace:   v.Namespace,
		Name:        v.Name,
		SlugName:    v.SlugName,
		Description: v.Description,
		Active:      v.Active,
		UpdatedAt:   timestamppb.New(v.UpdatedAt),
		Secrets:     secrets,
		Configmaps:  configmaps,
	}
}
