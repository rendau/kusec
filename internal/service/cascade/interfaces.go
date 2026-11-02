package cascade

import (
	"context"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
)

type AppServiceI interface {
	Delete(ctx context.Context, id string) error
}

type SecretServiceI interface {
	List(ctx context.Context, pars *secretModel.ListReq) ([]*secretModel.Main, int64, error)
	Delete(ctx context.Context, id string) error
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
	Delete(ctx context.Context, id string) error
}

type ConfigMapServiceI interface {
	List(ctx context.Context, pars *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
	Delete(ctx context.Context, id string) error
}

type ConfigItemServiceI interface {
	List(ctx context.Context, pars *configitemModel.ListReq) ([]*configitemModel.Main, int64, error)
	Delete(ctx context.Context, id string) error
}

type AuditRecorderI interface {
	RecordApp(ctx context.Context, old, cur *appModel.Main, action string, batchId *string) error
	RecordSecret(ctx context.Context, old, cur *secretModel.Main, action string, batchId *string) error
	RecordItem(ctx context.Context, old, cur *itemModel.Main, action string, batchId *string) error
	RecordConfigMap(ctx context.Context, old, cur *configmapModel.Main, action string, batchId *string) error
	RecordConfigItem(ctx context.Context, old, cur *configitemModel.Main, action string, batchId *string) error
}
