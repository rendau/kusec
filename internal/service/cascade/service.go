// Package cascade — удаление сущности вместе с потомками и записями аудита.
// FK удаляет потомков молча, поэтому запись на каждого ребёнка делается здесь
// (иначе пропажа ключей была бы бесследной).
//
// Методы не открывают транзакцию сами: вызывающий оборачивает их в TxFn
// (вложенный TxFn закоммитил бы внешнюю транзакцию) и задаёт общий batch_id.
package cascade

import (
	"context"
	"fmt"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
)

type Service struct {
	appSvc        AppServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI
	auditRec      AuditRecorderI
}

func New(
	appSvc AppServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
	auditRec AuditRecorderI,
) *Service {
	return &Service{
		appSvc:        appSvc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
		auditRec:      auditRec,
	}
}

func (s *Service) DeleteItem(ctx context.Context, cur *itemModel.Main, batchId *string) error {
	// запись аудита — до удаления, пока резолвится контекст secret/app
	if err := s.auditRec.RecordItem(ctx, cur, nil, "", batchId); err != nil {
		return fmt.Errorf("auditRec.RecordItem: %w", err)
	}
	if err := s.itemSvc.Delete(ctx, cur.Id); err != nil {
		return fmt.Errorf("itemSvc.Delete: %w", err)
	}
	return nil
}

func (s *Service) DeleteConfigItem(ctx context.Context, cur *configitemModel.Main, batchId *string) error {
	// запись аудита — до удаления, пока резолвится контекст configmap/app
	if err := s.auditRec.RecordConfigItem(ctx, cur, nil, "", batchId); err != nil {
		return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
	}
	if err := s.configItemSvc.Delete(ctx, cur.Id); err != nil {
		return fmt.Errorf("configItemSvc.Delete: %w", err)
	}
	return nil
}

func (s *Service) DeleteSecret(ctx context.Context, cur *secretModel.Main, batchId *string) error {
	items, _, err := s.itemSvc.List(ctx, &itemModel.ListReq{SecretId: new(cur.Id)})
	if err != nil {
		return fmt.Errorf("itemSvc.List: %w", err)
	}
	for _, item := range items {
		if err = s.auditRec.RecordItem(ctx, item, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordItem: %w", err)
		}
	}

	if err = s.secretSvc.Delete(ctx, cur.Id); err != nil {
		return fmt.Errorf("secretSvc.Delete: %w", err)
	}
	if err = s.auditRec.RecordSecret(ctx, cur, nil, "", batchId); err != nil {
		return fmt.Errorf("auditRec.RecordSecret: %w", err)
	}
	return nil
}

func (s *Service) DeleteConfigMap(ctx context.Context, cur *configmapModel.Main, batchId *string) error {
	configItems, _, err := s.configItemSvc.List(ctx, &configitemModel.ListReq{ConfigMapId: new(cur.Id)})
	if err != nil {
		return fmt.Errorf("configItemSvc.List: %w", err)
	}
	for _, configItem := range configItems {
		if err = s.auditRec.RecordConfigItem(ctx, configItem, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
		}
	}

	if err = s.configMapSvc.Delete(ctx, cur.Id); err != nil {
		return fmt.Errorf("configMapSvc.Delete: %w", err)
	}
	if err = s.auditRec.RecordConfigMap(ctx, cur, nil, "", batchId); err != nil {
		return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
	}
	return nil
}

func (s *Service) DeleteApp(ctx context.Context, cur *appModel.Main, batchId *string) error {
	if err := s.recordAppChildren(ctx, cur.Id, batchId); err != nil {
		return err
	}

	if err := s.appSvc.Delete(ctx, cur.Id); err != nil {
		return fmt.Errorf("appSvc.Delete: %w", err)
	}
	if err := s.auditRec.RecordApp(ctx, cur, nil, "", batchId); err != nil {
		return fmt.Errorf("auditRec.RecordApp: %w", err)
	}
	return nil
}

func (s *Service) recordAppChildren(ctx context.Context, appId string, batchId *string) error {
	secrets, _, err := s.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("secretSvc.List: %w", err)
	}
	items, _, err := s.itemSvc.List(ctx, &itemModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("itemSvc.List: %w", err)
	}
	for _, item := range items {
		if err = s.auditRec.RecordItem(ctx, item, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordItem: %w", err)
		}
	}
	for _, secret := range secrets {
		if err = s.auditRec.RecordSecret(ctx, secret, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordSecret: %w", err)
		}
	}

	configMaps, _, err := s.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("configMapSvc.List: %w", err)
	}
	configItems, _, err := s.configItemSvc.List(ctx, &configitemModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("configItemSvc.List: %w", err)
	}
	for _, configItem := range configItems {
		if err = s.auditRec.RecordConfigItem(ctx, configItem, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
		}
	}
	for _, configMap := range configMaps {
		if err = s.auditRec.RecordConfigMap(ctx, configMap, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
		}
	}

	return nil
}
