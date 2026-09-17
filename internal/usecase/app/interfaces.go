package app

import (
	"context"

	"github.com/rendau/kusec/internal/domain/app/model"
	configitemModel "github.com/rendau/kusec/internal/domain/configitem/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	itemModel "github.com/rendau/kusec/internal/domain/item/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Main, bool, error)
	Create(ctx context.Context, obj *model.Edit) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Delete(ctx context.Context, id string) error
}

// Дочерние сущности нужны только для записей аудита при каскадном удалении
// app (FK удаляет их молча — пропажа ключей была бы бесследной).

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

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

type AuditRecorderI interface {
	NewBatchId() string
	RecordApp(ctx context.Context, old, cur *model.Main, action string, batchId *string) error
	RecordSecret(ctx context.Context, old, cur *secretModel.Main, action string, batchId *string) error
	RecordItem(ctx context.Context, old, cur *itemModel.Main, action string, batchId *string) error
	RecordConfigMap(ctx context.Context, old, cur *configmapModel.Main, action string, batchId *string) error
	RecordConfigItem(ctx context.Context, old, cur *configitemModel.Main, action string, batchId *string) error
}
