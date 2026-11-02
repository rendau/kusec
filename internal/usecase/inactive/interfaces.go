package inactive

import (
	"context"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
)

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*appModel.Main, bool, error)
}

type SecretServiceI interface {
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*secretModel.Main, bool, error)
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*itemModel.Main, bool, error)
}

type ConfigMapServiceI interface {
	List(ctx context.Context, pars *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*configmapModel.Main, bool, error)
}

type ConfigItemServiceI interface {
	List(ctx context.Context, pars *configitemModel.ListReq) ([]*configitemModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*configitemModel.Main, bool, error)
}

type CascadeServiceI interface {
	DeleteApp(ctx context.Context, cur *appModel.Main, batchId *string) error
	DeleteSecret(ctx context.Context, cur *secretModel.Main, batchId *string) error
	DeleteItem(ctx context.Context, cur *itemModel.Main, batchId *string) error
	DeleteConfigMap(ctx context.Context, cur *configmapModel.Main, batchId *string) error
	DeleteConfigItem(ctx context.Context, cur *configitemModel.Main, batchId *string) error
}

type SessionServiceI interface {
	CtxIsAdmin(ctx context.Context) bool
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

type AuditRecorderI interface {
	NewBatchId() string
}
