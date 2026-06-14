package kube

import (
	"context"

	appModel "github.com/mechta-market/kusec/internal/domain/app/model"
	configitemModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
	configmapModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
	itemModel "github.com/mechta-market/kusec/internal/domain/item/model"
	secretModel "github.com/mechta-market/kusec/internal/domain/secret/model"
)

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*appModel.Main, bool, error)
}

type SecretServiceI interface {
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
	Create(ctx context.Context, obj *secretModel.Edit) (string, error)
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
	Create(ctx context.Context, obj *itemModel.Edit) (string, error)
}

type ConfigMapServiceI interface {
	List(ctx context.Context, pars *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
}

type ConfigItemServiceI interface {
	List(ctx context.Context, pars *configitemModel.ListReq) ([]*configitemModel.Main, int64, error)
}
