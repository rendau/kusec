package audit

import (
	"context"
	"fmt"

	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/util"
)

// Usecase — чтение аудита. Записи через API не редактируются и не
// удаляются: мутирующих методов нет намеренно.
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

func (u *Usecase) List(ctx context.Context, pars *auditModel.ListReq) ([]*auditModel.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
		return nil, 0, err
	}

	// не-админ видит только записи своих app (записи без app_id — например
	// по api_key/usr — ему не показываются)
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
