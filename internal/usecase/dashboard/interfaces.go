package dashboard

import (
	"context"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	usrModel "github.com/mechta-market/kusec/internal/domain/usr/model"
)

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*appModel.Main, bool, error)
}

type SecretServiceI interface {
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
}

type UsrServiceI interface {
	List(ctx context.Context, pars *usrModel.ListReq) ([]*usrModel.Main, int64, error)
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
}
