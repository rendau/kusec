package transfer

import (
	"context"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
)

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
}

type SecretServiceI interface {
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
}

type ConfigMapServiceI interface {
	List(ctx context.Context, pars *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
}

type ConfigItemServiceI interface {
	List(ctx context.Context, pars *configitemModel.ListReq) ([]*configitemModel.Main, int64, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
