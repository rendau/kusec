package transfer

import (
	"context"
	"fmt"
	"sort"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	configitemModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	"github.com/mechta-market/kusec/internal/errs"
)

type Usecase struct {
	appSvc        AppServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI
	sessionSvc    SessionServiceI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
	sessionSvc SessionServiceI,
) *Usecase {
	return &Usecase{
		appSvc:        appSvc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
		sessionSvc:    sessionSvc,
	}
}

// Tree возвращает все записи деревом app → secret → item без значений
// item-ов (вместо value — его размер). Требуется аутентификация (любой
// пользователь, права администратора не нужны).
func (u *Usecase) Tree(ctx context.Context) ([]*TreeApp, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	accessibleAppIds, all := u.sessionSvc.FromContext(ctx).AccessibleAppIds()

	listReq := &appModel.ListReq{}
	if !all {
		listReq.Ids = accessibleAppIds
	}

	apps, _, err := u.appSvc.List(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Namespace != apps[j].Namespace {
			return apps[i].Namespace < apps[j].Namespace
		}
		return apps[i].SlugName < apps[j].SlugName
	})

	tree := make([]*TreeApp, 0, len(apps))
	for _, app := range apps {
		secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(app.Id)})
		if err != nil {
			return nil, fmt.Errorf("secretSvc.List: %w", err)
		}
		sort.Slice(secrets, func(i, j int) bool {
			return secrets[i].SlugName < secrets[j].SlugName
		})

		treeApp := &TreeApp{
			Id:          app.Id,
			Namespace:   app.Namespace,
			Name:        app.Name,
			SlugName:    app.SlugName,
			Description: app.Description,
			Active:      app.Active,
			UpdatedAt:   app.UpdatedAt,
			Secrets:     make([]*TreeSecret, 0, len(secrets)),
		}

		for _, secret := range secrets {
			items, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{SecretId: new(secret.Id)})
			if err != nil {
				return nil, fmt.Errorf("itemSvc.List: %w", err)
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })

			treeSecret := &TreeSecret{
				Id:          secret.Id,
				SlugName:    secret.SlugName,
				Description: secret.Description,
				Active:      secret.Active,
				KubeType:    secret.KubeType,
				UpdatedAt:   secret.UpdatedAt,
				Items:       make([]*TreeItem, 0, len(items)),
			}
			for _, item := range items {
				treeSecret.Items = append(treeSecret.Items, &TreeItem{
					Id:          item.Id,
					Key:         item.Key,
					ValueFormat: item.ValueFormat,
					Encoding:    item.Encoding,
					FileName:    item.FileName,
					ContentType: item.ContentType,
					Description: item.Description,
					Active:      item.Active,
					UpdatedAt:   item.UpdatedAt,
					// Значение скрыто; размер хранимой строки в байтах.
					ValueSize: int64(len(item.Value)),
				})
			}
			treeApp.Secrets = append(treeApp.Secrets, treeSecret)
		}

		configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: new(app.Id)})
		if err != nil {
			return nil, fmt.Errorf("configMapSvc.List: %w", err)
		}
		sort.Slice(configMaps, func(i, j int) bool {
			return configMaps[i].SlugName < configMaps[j].SlugName
		})

		treeApp.ConfigMaps = make([]*TreeConfigMap, 0, len(configMaps))
		for _, configMap := range configMaps {
			items, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{ConfigMapId: new(configMap.Id)})
			if err != nil {
				return nil, fmt.Errorf("configItemSvc.List: %w", err)
			}
			sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })

			treeConfigMap := &TreeConfigMap{
				Id:          configMap.Id,
				SlugName:    configMap.SlugName,
				Description: configMap.Description,
				Active:      configMap.Active,
				UpdatedAt:   configMap.UpdatedAt,
				Items:       make([]*TreeConfigItem, 0, len(items)),
			}
			for _, item := range items {
				treeConfigMap.Items = append(treeConfigMap.Items, &TreeConfigItem{
					Id:          item.Id,
					Key:         item.Key,
					ValueFormat: item.ValueFormat,
					Encoding:    item.Encoding,
					FileName:    item.FileName,
					ContentType: item.ContentType,
					Description: item.Description,
					Active:      item.Active,
					UpdatedAt:   item.UpdatedAt,
					// Значение скрыто; размер хранимой строки в байтах.
					ValueSize: int64(len(item.Value)),
				})
			}
			treeApp.ConfigMaps = append(treeApp.ConfigMaps, treeConfigMap)
		}

		tree = append(tree, treeApp)
	}

	return tree, nil
}
