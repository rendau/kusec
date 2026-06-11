package dto

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	usecaseModel "github.com/mechta-market/kusec/internal/usecase/transfer"
	proto "github.com/mechta-market/kusec/pkg/proto/kusec_v1"
)

// proto → usecase

func DecodeTransferImportReq(v *proto.TransferImportReq) *usecaseModel.ImportReq {
	if v == nil {
		return nil
	}
	apps := make([]*usecaseModel.ImportApp, 0, len(v.Apps))
	for _, app := range v.Apps {
		if app == nil {
			continue
		}
		secrets := make([]*usecaseModel.ImportSecret, 0, len(app.Secrets))
		for _, secret := range app.Secrets {
			if secret == nil {
				continue
			}
			items := make([]*usecaseModel.ImportItem, 0, len(secret.Items))
			for _, item := range secret.Items {
				if item == nil {
					continue
				}
				items = append(items, &usecaseModel.ImportItem{
					Key:         item.Key,
					Value:       item.Value,
					ValueFormat: item.ValueFormat,
					Encoding:    item.Encoding,
					FileName:    item.FileName,
					ContentType: item.ContentType,
					Description: item.Description,
					Active:      item.Active,
				})
			}
			secrets = append(secrets, &usecaseModel.ImportSecret{
				SlugName:    secret.SlugName,
				Description: secret.Description,
				Active:      secret.Active,
				Items:       items,
			})
		}
		apps = append(apps, &usecaseModel.ImportApp{
			Namespace:   app.Namespace,
			Name:        app.Name,
			SlugName:    app.SlugName,
			Description: app.Description,
			Active:      app.Active,
			Secrets:     secrets,
		})
	}
	return &usecaseModel.ImportReq{Apps: apps}
}

// usecase → proto

func EncodeTransferImportResult(v *usecaseModel.ImportResult) *proto.TransferImportRep {
	if v == nil {
		return nil
	}
	return &proto.TransferImportRep{
		AppsCreated:    v.AppsCreated,
		AppsUpdated:    v.AppsUpdated,
		SecretsCreated: v.SecretsCreated,
		SecretsUpdated: v.SecretsUpdated,
		ItemsCreated:   v.ItemsCreated,
		ItemsUpdated:   v.ItemsUpdated,
		Unchanged:      v.Unchanged,
		Errors:         v.Errors,
	}
}

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
			UpdatedAt:   timestamppb.New(secret.UpdatedAt),
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
	}
}
