package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	commonRepoPg "github.com/rendau/kusec/internal/domain/common/repo/pg"
	"github.com/rendau/kusec/internal/domain/syncrun/model"
	repoModel "github.com/rendau/kusec/internal/domain/syncrun/repo/db/model"
)

type Repo struct {
	*commonRepoPg.Base
	ModelStore       *mobone.ModelStore
	ObjectModelStore *mobone.ModelStore
}

func New(con *pgxpool.Pool) *Repo {
	base := commonRepoPg.NewBase(con)
	return &Repo{
		Base: base,
		ModelStore: &mobone.ModelStore{
			Con:                base.Con,
			QB:                 base.QB,
			TransactionManager: base.TxM,
			TableName:          "sync_run",
		},
		ObjectModelStore: &mobone.ModelStore{
			Con:                base.Con,
			QB:                 base.QB,
			TransactionManager: base.TxM,
			TableName:          "sync_run_object",
		},
	}
}

func (r *Repo) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	conditions, conditionExps := r.getConditions(pars)
	sort := moboneTools.ConstructSortColumns(allowedSortFields, pars.Sort)
	items := make([]*repoModel.Select, 0)

	totalCount, err := r.ModelStore.List(ctx, mobone.ListParams{
		Conditions:           conditions,
		ConditionExpressions: conditionExps,
		Page:                 pars.Page,
		PageSize:             pars.PageSize,
		WithTotalCount:       pars.WithTotalCount,
		OnlyCount:            pars.OnlyCount,
		Sort:                 sort,
	}, func(add bool) mobone.ListModelI {
		item := &repoModel.Select{}
		if add {
			items = append(items, item)
		}
		return item
	})
	if err != nil {
		return nil, 0, fmt.Errorf("ModelStore.List: %w", err)
	}
	return lo.Map(items, repoModel.EncodeSelect), totalCount, nil
}

func (r *Repo) Get(ctx context.Context, id string) (*model.Main, bool, error) {
	m := &repoModel.Select{Id: id}
	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, false, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	return repoModel.EncodeSelect(m, 0), true, nil
}

func (r *Repo) Create(ctx context.Context, obj *model.Main) (string, error) {
	m := repoModel.DecodeCreate(obj)
	if err := r.ModelStore.Create(ctx, m); err != nil {
		return "", fmt.Errorf("ModelStore.Create: %w", err)
	}
	return m.NewId, nil
}

func (r *Repo) Update(ctx context.Context, id string, obj *model.Edit) error {
	m := repoModel.DecodeEdit(obj)
	m.PKId = id
	if err := r.ModelStore.Update(ctx, m); err != nil {
		return fmt.Errorf("ModelStore.Update: %w", err)
	}
	return nil
}

// CreateObjects добавляет затронутые k8s-объекты запуска (батчем).
func (r *Repo) CreateObjects(ctx context.Context, runId string, objects []*model.Object) error {
	if len(objects) == 0 {
		return nil
	}

	models := lo.Map(objects, func(v *model.Object, _ int) mobone.CreateModelI {
		m := repoModel.DecodeObjectUpsert(v)
		m.RunId = runId
		return m
	})
	if err := r.ObjectModelStore.CreateMany(ctx, models); err != nil {
		return fmt.Errorf("ObjectModelStore.CreateMany: %w", err)
	}
	return nil
}

// ListObjects возвращает объекты запусков (без пагинации: объектов одного
// запуска немного).
func (r *Repo) ListObjects(ctx context.Context, runIds []string) ([]*model.Object, error) {
	if len(runIds) == 0 {
		return []*model.Object{}, nil
	}

	items := make([]*repoModel.ObjectSelect, 0)
	_, err := r.ObjectModelStore.List(ctx, mobone.ListParams{
		Conditions: map[string]any{"run_id": runIds},
	}, func(add bool) mobone.ListModelI {
		item := &repoModel.ObjectSelect{}
		if add {
			items = append(items, item)
		}
		return item
	})
	if err != nil {
		return nil, fmt.Errorf("ObjectModelStore.List: %w", err)
	}
	return lo.Map(items, repoModel.EncodeObjectSelect), nil
}
