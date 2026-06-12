package secret

import (
	"context"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	"github.com/mechta-market/kusec/internal/domain/secret/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Main, bool, error)
	Create(ctx context.Context, obj *model.Edit) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Delete(ctx context.Context, id string) error
}

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
