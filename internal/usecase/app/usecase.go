package app

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/rendau/kusec/internal/domain/app/model"
	"github.com/rendau/kusec/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(svc ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
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
	newId, err := u.svc.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("svc.Create: %w", err)
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
	if err := u.svc.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}
	return nil
}

func (u *Usecase) Delete(ctx context.Context, id string) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if id == "" {
		return errs.IdRequired
	}
	if err := u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}
	return nil
}
