package secret

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	"github.com/mechta-market/kusec/internal/domain/secret/model"
	"github.com/mechta-market/kusec/internal/errs"
	"github.com/mechta-market/kusec/internal/service/kube"
	"github.com/mechta-market/kusec/internal/util"
)

type Usecase struct {
	svc        ServiceI
	appSvc     AppServiceI
	sessionSvc SessionServiceI
}

func New(svc ServiceI, appSvc AppServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		appSvc:     appSvc,
		sessionSvc: sessionSvc,
	}
}

func (u *Usecase) fillKubeSecretName(ctx context.Context, items []*model.Main) error {
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
			item.KubeSecretName = kube.SecretName(appSlug, item.SlugName)
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
	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}
	if err = u.fillKubeSecretName(ctx, items); err != nil {
		return nil, 0, fmt.Errorf("fillKubeSecretName: %w", err)
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
	if err = u.fillKubeSecretName(ctx, []*model.Main{result}); err != nil {
		return nil, fmt.Errorf("fillKubeSecretName: %w", err)
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
