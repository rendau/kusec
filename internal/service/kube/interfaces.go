package kube

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
	Create(ctx context.Context, obj *secretModel.Edit) (string, error)
}

type ItemServiceI interface {
	List(ctx context.Context, pars *itemModel.ListReq) ([]*itemModel.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*itemModel.Main, bool, error)
	Create(ctx context.Context, obj *itemModel.Edit) (string, error)
	Update(ctx context.Context, id string, obj *itemModel.Edit) error
}

type ConfigMapServiceI interface {
	List(ctx context.Context, pars *configmapModel.ListReq) ([]*configmapModel.Main, int64, error)
}

type ConfigItemServiceI interface {
	List(ctx context.Context, pars *configitemModel.ListReq) ([]*configitemModel.Main, int64, error)
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

// AuditRecorderI — порт регистратора аудита (internal/service/audit);
// объявлен здесь, чтобы не создавать цикл импортов (регистратор сам
// использует этот пакет для имён k8s-объектов).
type AuditRecorderI interface {
	NewBatchId() string
	RecordSecret(ctx context.Context, old, cur *secretModel.Main, action string, batchId *string) error
	RecordItem(ctx context.Context, old, cur *itemModel.Main, action string, batchId *string) error
	RecordSyncRun(ctx context.Context, runId string, appId *string) error
}
