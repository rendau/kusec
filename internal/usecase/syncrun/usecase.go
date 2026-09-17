package syncrun

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/util"
)

// Usecase — чтение журнала sync. Записи через API не редактируются и не
// удаляются: мутирующих методов нет намеренно.
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

func (u *Usecase) List(ctx context.Context, pars *syncrunModel.ListReq) ([]*syncrunModel.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
		return nil, 0, err
	}

	// не-админ видит запуски своих app и глобальные (app_id is null)
	session := u.sessionSvc.FromContext(ctx)
	if appIds, all := session.AccessibleAppIds(); !all {
		if pars.AppId != nil {
			if !session.HasAppAccess(*pars.AppId) {
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
	return items, tCount, nil
}

// Get возвращает запуск с объектами. Для не-админа объекты глобального
// запуска фильтруются по namespace-ам его приложений.
func (u *Usecase) Get(ctx context.Context, id string) (*syncrunModel.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if id == "" {
		return nil, errs.IdRequired
	}

	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	session := u.sessionSvc.FromContext(ctx)
	appIds, all := session.AccessibleAppIds()
	if all {
		return result, nil
	}

	if result.AppId != nil && !session.HasAppAccess(*result.AppId) {
		return nil, errs.NoPermission
	}

	// глобальный запуск: объекты чужих namespace-ов скрываются
	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{Ids: appIds})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	namespaces := lo.SliceToMap(apps, func(app *appModel.Main) (string, bool) { return app.Namespace, true })

	result.Objects = lo.Filter(result.Objects, func(obj *syncrunModel.Object, _ int) bool {
		return namespaces[obj.Namespace]
	})

	return result, nil
}
