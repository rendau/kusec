package app

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	"github.com/rendau/kusec/internal/errs"
)

type Usecase struct {
	svc           ServiceI
	secretSvc     SecretServiceI
	itemSvc       ItemServiceI
	configMapSvc  ConfigMapServiceI
	configItemSvc ConfigItemServiceI
	sessionSvc    SessionServiceI
	txm           TransactionManagerI
	auditRec      AuditRecorderI
}

func New(
	svc ServiceI,
	secretSvc SecretServiceI,
	itemSvc ItemServiceI,
	configMapSvc ConfigMapServiceI,
	configItemSvc ConfigItemServiceI,
	sessionSvc SessionServiceI,
	txm TransactionManagerI,
	auditRec AuditRecorderI,
) *Usecase {
	return &Usecase{
		svc:           svc,
		secretSvc:     secretSvc,
		itemSvc:       itemSvc,
		configMapSvc:  configMapSvc,
		configItemSvc: configItemSvc,
		sessionSvc:    sessionSvc,
		txm:           txm,
		auditRec:      auditRec,
	}
}

func (u *Usecase) accessibleAppIds(ctx context.Context) ([]string, bool) {
	return u.sessionSvc.FromContext(ctx).AccessibleAppIds()
}

func (u *Usecase) validateEdit(obj *model.Edit, forCreate bool) error {
	if forCreate {
		if obj.Name == nil || *obj.Name == "" {
			return errs.InvalidRequest
		}
		if obj.SlugName == nil || *obj.SlugName == "" {
			return errs.InvalidRequest
		}
	}
	if obj.Name != nil && *obj.Name == "" {
		return errs.InvalidRequest
	}
	if obj.SlugName != nil && *obj.SlugName == "" {
		return errs.InvalidRequest
	}
	return nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}

	appIds, all := u.accessibleAppIds(ctx)
	if !all {
		if len(pars.Ids) == 0 {
			pars.Ids = appIds
		} else {
			pars.Ids = lo.Intersect(pars.Ids, appIds)
		}
		if len(pars.Ids) == 0 {
			return []*model.Main{}, 0, nil
		}
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}
	return items, tCount, nil
}

func (u *Usecase) Get(ctx context.Context, id string) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	appIds, all := u.accessibleAppIds(ctx)
	if !all && !lo.Contains(appIds, id) {
		return nil, errs.NoPermission
	}

	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	return result, nil
}

func (u *Usecase) Create(ctx context.Context, obj *model.Edit) (string, error) {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return "", errs.NoPermission
	}
	if err := u.validateEdit(obj, true); err != nil {
		return "", err
	}

	var newId string
	err := u.txm.TxFn(ctx, func(ctx context.Context) error {
		var err error
		newId, err = u.svc.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("svc.Create: %w", err)
		}

		created, _, err := u.svc.Get(ctx, newId, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordApp(ctx, nil, created, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordApp: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return newId, nil
}

func (u *Usecase) Update(ctx context.Context, id string, obj *model.Edit) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if id == "" {
		return errs.IdRequired
	}
	if err := u.validateEdit(obj, false); err != nil {
		return err
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		current, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}

		if err = u.svc.Update(ctx, id, obj); err != nil {
			return fmt.Errorf("svc.Update: %w", err)
		}

		updated, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordApp(ctx, current, updated, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordApp: %w", err)
		}
		return nil
	})
}

// Delete удаляет app; дочерние secret/item/configmap/config_item удаляет
// каскад FK, поэтому записи аудита на каждого ребёнка пишутся здесь явно
// (с общим batch_id) — до удаления, пока контекст объекта ещё резолвится.
func (u *Usecase) Delete(ctx context.Context, id string) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if id == "" {
		return errs.IdRequired
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		current, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}

		batchId := new(u.auditRec.NewBatchId())

		if err = u.recordChildrenDeletion(ctx, id, batchId); err != nil {
			return err
		}

		if err = u.svc.Delete(ctx, id); err != nil {
			return fmt.Errorf("svc.Delete: %w", err)
		}
		if err = u.auditRec.RecordApp(ctx, current, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordApp: %w", err)
		}
		return nil
	})
}

// recordChildrenDeletion пишет записи аудита на удаление всех дочерних
// сущностей app (иначе пропажа ключей была бы бесследной).
func (u *Usecase) recordChildrenDeletion(ctx context.Context, appId string, batchId *string) error {
	secrets, _, err := u.secretSvc.List(ctx, &secretModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("secretSvc.List: %w", err)
	}
	items, _, err := u.itemSvc.List(ctx, &itemModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("itemSvc.List: %w", err)
	}
	for _, item := range items {
		if err = u.auditRec.RecordItem(ctx, item, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordItem: %w", err)
		}
	}
	for _, secret := range secrets {
		if err = u.auditRec.RecordSecret(ctx, secret, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordSecret: %w", err)
		}
	}

	configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("configMapSvc.List: %w", err)
	}
	configItems, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{AppId: new(appId)})
	if err != nil {
		return fmt.Errorf("configItemSvc.List: %w", err)
	}
	for _, configItem := range configItems {
		if err = u.auditRec.RecordConfigItem(ctx, configItem, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
		}
	}
	for _, configMap := range configMaps {
		if err = u.auditRec.RecordConfigMap(ctx, configMap, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
		}
	}

	return nil
}
