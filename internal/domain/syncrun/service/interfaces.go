package service

import (
	"context"
	"time"

	"github.com/rendau/kusec/internal/domain/syncrun/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id string) (*model.Main, bool, error)
	Create(ctx context.Context, obj *model.Main) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	CreateObjects(ctx context.Context, runId string, objects []*model.Object) error
	ListObjects(ctx context.Context, runIds []string) ([]*model.Object, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}
