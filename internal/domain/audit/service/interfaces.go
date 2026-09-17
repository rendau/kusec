package service

import (
	"context"
	"time"

	"github.com/rendau/kusec/internal/domain/audit/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Create(ctx context.Context, obj *model.Main) error
	LastEditors(ctx context.Context, appId string) (map[string]string, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}
