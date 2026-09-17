package syncrun

import (
	"context"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *syncrunModel.ListReq) ([]*syncrunModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*syncrunModel.Main, bool, error)
}

// AppServiceI нужен для фильтрации объектов глобального запуска по
// namespace-ам доступных приложений (не-админ).
type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
}
