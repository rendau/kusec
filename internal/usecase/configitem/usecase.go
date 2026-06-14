package configitem

import (
	"context"
	"fmt"

	"github.com/mechta-market/kusec/internal/domain/configitem/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	"github.com/mechta-market/kusec/internal/errs"
	"github.com/mechta-market/kusec/internal/util"
)

type Usecase struct {
	svc          ServiceI
	configMapSvc ConfigMapServiceI
	sessionSvc   SessionServiceI
}

func New(svc ServiceI, configMapSvc ConfigMapServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:          svc,
		configMapSvc: configMapSvc,
		sessionSvc:   sessionSvc,
	}
}

func (u *Usecase) requireConfigMapAccess(ctx context.Context, configMapId string) error {
	session := u.sessionSvc.FromContext(ctx)
	if _, all := session.AccessibleAppIds(); all {
		return nil
	}

	configMap, _, err := u.configMapSvc.Get(ctx, configMapId, true)
	if err != nil {
		return fmt.Errorf("configMapSvc.Get: %w", err)
	}
	if !session.HasAppAccess(configMap.AppId) {
		return errs.NoPermission
	}
	return nil
}

// requireConfigMapsAccess проверяет доступ ко всем configmap-ам одним запросом:
// каждый запрошенный id должен существовать и принадлежать доступному app.
func (u *Usecase) requireConfigMapsAccess(ctx context.Context, configMapIds []string) error {
	session := u.sessionSvc.FromContext(ctx)
	if _, all := session.AccessibleAppIds(); all {
		return nil
	}

	configMaps, _, err := u.configMapSvc.List(ctx, &configmapModel.ListReq{Ids: configMapIds})
	if err != nil {
		return fmt.Errorf("configMapSvc.List: %w", err)
	}

	appByConfigMap := make(map[string]string, len(configMaps))
	for _, configMap := range configMaps {
		appByConfigMap[configMap.Id] = configMap.AppId
	}

	for _, id := range configMapIds {
		appId, ok := appByConfigMap[id]
		if !ok || !session.HasAppAccess(appId) {
			return errs.NoPermission
		}
	}
	return nil
}

func (u *Usecase) validateEdit(obj *model.Edit, forCreate bool) error {
	if forCreate {
		if obj.ConfigMapId == nil || *obj.ConfigMapId == "" {
			return errs.InvalidRequest
		}
		if obj.Key == nil || *obj.Key == "" {
			return errs.InvalidRequest
		}
	}
	if obj.ConfigMapId != nil && *obj.ConfigMapId == "" {
		return errs.InvalidRequest
	}
	if obj.Key != nil && *obj.Key == "" {
		return errs.InvalidRequest
	}
	return nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	// Запрос считается узким (без обязательной пагинации), если ограничен
	// одним configmap или набором configmap-ов.
	scopedByConfigMap := pars.ConfigMapId != nil || len(pars.ConfigMapIds) > 0
	if !scopedByConfigMap {
		if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
			return nil, 0, err
		}
	}

	if _, all := u.sessionSvc.FromContext(ctx).AccessibleAppIds(); !all {
		if !scopedByConfigMap {
			return nil, 0, errs.NoPermission
		}
		if pars.ConfigMapId != nil {
			if err := u.requireConfigMapAccess(ctx, *pars.ConfigMapId); err != nil {
				return nil, 0, err
			}
		}
		if len(pars.ConfigMapIds) > 0 {
			if err := u.requireConfigMapsAccess(ctx, pars.ConfigMapIds); err != nil {
				return nil, 0, err
			}
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
	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireConfigMapAccess(ctx, result.ConfigMapId); err != nil {
		return nil, err
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
	if err := u.requireConfigMapAccess(ctx, *obj.ConfigMapId); err != nil {
		return "", err
	}
	newId, err := u.svc.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("svc.Create: %w", err)
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
	if err = u.requireConfigMapAccess(ctx, current.ConfigMapId); err != nil {
		return err
	}
	if obj.ConfigMapId != nil && *obj.ConfigMapId != current.ConfigMapId {
		if err = u.requireConfigMapAccess(ctx, *obj.ConfigMapId); err != nil {
			return err
		}
	}

	if err = u.svc.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}
	return nil
}

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
	if err = u.requireConfigMapAccess(ctx, current.ConfigMapId); err != nil {
		return err
	}

	if err = u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}
	return nil
}
