package transfer

import (
	"context"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
)

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
	Create(ctx context.Context, obj *appModel.Edit) (string, error)
	Update(ctx context.Context, id string, obj *appModel.Edit) error
}

type SecretServiceI interface {
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
	Create(ctx context.Context, obj *secretModel.Edit) (string, error)
	Update(ctx context.Context, id string, obj *secretModel.Edit) error
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
	Create(ctx context.Context, obj *itemModel.Edit) (string, error)
	Update(ctx context.Context, id string, obj *itemModel.Edit) error
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
