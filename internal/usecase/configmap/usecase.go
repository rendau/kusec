package configmap

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	"github.com/rendau/kusec/internal/domain/configmap/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/service/kube"
	"github.com/rendau/kusec/internal/util"
)

type Usecase struct {
	svc           ServiceI
	appSvc        AppServiceI
	configItemSvc ConfigItemServiceI
	sessionSvc    SessionServiceI
	txm           TransactionManagerI
	auditRec      AuditRecorderI
}

func New(
	svc ServiceI,
	appSvc AppServiceI,
	configItemSvc ConfigItemServiceI,
	sessionSvc SessionServiceI,
	txm TransactionManagerI,
	auditRec AuditRecorderI,
) *Usecase {
	return &Usecase{
		svc:           svc,
		appSvc:        appSvc,
		configItemSvc: configItemSvc,
		sessionSvc:    sessionSvc,
		txm:           txm,
		auditRec:      auditRec,
	}
}

func (u *Usecase) accessibleAppIds(ctx context.Context) ([]string, bool) {
	return u.sessionSvc.FromContext(ctx).AccessibleAppIds()
}

func (u *Usecase) requireAppAccess(ctx context.Context, appId string) error {
	if !u.sessionSvc.FromContext(ctx).HasAppAccess(appId) {
		return errs.NoPermission
	}
	return nil
}

func (u *Usecase) fillKubeConfigMapName(ctx context.Context, items []*model.Main) error {
	if len(items) == 0 {
		return nil
	}

	appIds := lo.Uniq(lo.Map(items, func(item *model.Main, _ int) string {
		return item.AppId
	}))

	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{Ids: appIds})
	if err != nil {
		return fmt.Errorf("appSvc.List: %w", err)
	}

	appSlugs := lo.SliceToMap(apps, func(app *appModel.Main) (string, string) {
		return app.Id, app.SlugName
	})

	for _, item := range items {
		if appSlug, ok := appSlugs[item.AppId]; ok {
			item.KubeConfigMapName = kube.ConfigMapName(appSlug, item.SlugName, item.ExactSlug)
		}
	}

	return nil
}

func (u *Usecase) validateEdit(obj *model.Edit, forCreate bool) error {
	if forCreate {
		if obj.AppId == nil || *obj.AppId == "" {
			return errs.InvalidRequest
		}
		if obj.SlugName == nil || *obj.SlugName == "" {
			return errs.InvalidRequest
		}
	}
	if obj.AppId != nil && *obj.AppId == "" {
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
	if pars.AppId == nil {
		if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
			return nil, 0, err
		}
	}

	appIds, all := u.accessibleAppIds(ctx)
	if !all {
		if pars.AppId != nil {
			if !lo.Contains(appIds, *pars.AppId) {
				return nil, 0, errs.NoPermission
			}
		} else {
			pars.AppIds = appIds
		}
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}
	if err = u.fillKubeConfigMapName(ctx, items); err != nil {
		return nil, 0, fmt.Errorf("fillKubeConfigMapName: %w", err)
	}
	return items, tCount, nil
}

func (u *Usecase) Get(ctx context.Context, id string) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireAppAccess(ctx, result.AppId); err != nil {
		return nil, err
	}
	if err = u.fillKubeConfigMapName(ctx, []*model.Main{result}); err != nil {
		return nil, fmt.Errorf("fillKubeConfigMapName: %w", err)
	}
	return result, nil
}

func (u *Usecase) Create(ctx context.Context, obj *model.Edit) (string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return "", errs.NotAuthorized
	}
	if err := u.validateEdit(obj, true); err != nil {
		return "", err
	}
	if err := u.requireAppAccess(ctx, *obj.AppId); err != nil {
		return "", err
	}
	// Включение exact_slug при создании = изменение флага, доступно только админам.
	if obj.ExactSlug != nil && *obj.ExactSlug && !u.sessionSvc.CtxIsAdmin(ctx) {
		return "", errs.NoPermission
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
		if err = u.auditRec.RecordConfigMap(ctx, nil, created, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return newId, nil
}

func (u *Usecase) Update(ctx context.Context, id string, obj *model.Edit) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	if id == "" {
		return errs.IdRequired
	}
	if err := u.validateEdit(obj, false); err != nil {
		return err
	}

	current, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireAppAccess(ctx, current.AppId); err != nil {
		return err
	}
	if obj.AppId != nil && *obj.AppId != current.AppId {
		if err = u.requireAppAccess(ctx, *obj.AppId); err != nil {
			return err
		}
	}
	// Менять флаг exact_slug могут только админы.
	if obj.ExactSlug != nil && *obj.ExactSlug != current.ExactSlug && !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		if err = u.svc.Update(ctx, id, obj); err != nil {
			return fmt.Errorf("svc.Update: %w", err)
		}

		updated, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordConfigMap(ctx, current, updated, "", nil); err != nil {
			return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
		}
		return nil
	})
}

// Delete удаляет configmap; config_item-ы удаляет каскад FK, записи аудита
// на каждый пишутся здесь явно (общий batch_id) — до удаления.
func (u *Usecase) Delete(ctx context.Context, id string) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	if id == "" {
		return errs.IdRequired
	}

	current, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireAppAccess(ctx, current.AppId); err != nil {
		return err
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		batchId := new(u.auditRec.NewBatchId())

		configItems, _, err := u.configItemSvc.List(ctx, &configitemModel.ListReq{ConfigMapId: new(id)})
		if err != nil {
			return fmt.Errorf("configItemSvc.List: %w", err)
		}
		for _, configItem := range configItems {
			if err = u.auditRec.RecordConfigItem(ctx, configItem, nil, "", batchId); err != nil {
				return fmt.Errorf("auditRec.RecordConfigItem: %w", err)
			}
		}

		if err = u.svc.Delete(ctx, id); err != nil {
			return fmt.Errorf("svc.Delete: %w", err)
		}
		if err = u.auditRec.RecordConfigMap(ctx, current, nil, "", batchId); err != nil {
			return fmt.Errorf("auditRec.RecordConfigMap: %w", err)
		}
		return nil
	})
}
