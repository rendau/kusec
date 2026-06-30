package configitem

import (
	"context"

	"github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Main, bool, error)
	Create(ctx context.Context, obj *model.Edit) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Delete(ctx context.Context, id string) error
}

type ConfigMapServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*configmapModel.Main, bool, error)
	List(ctx context.Context, pars *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
