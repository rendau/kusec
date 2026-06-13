package item

import (
	"context"

	"github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Main, bool, error)
	Create(ctx context.Context, obj *model.Edit) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Delete(ctx context.Context, id string) error
}

type SecretServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*secretModel.Main, bool, error)
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
