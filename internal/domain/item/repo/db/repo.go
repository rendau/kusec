package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	commonRepoPg "github.com/rendau/kusec/internal/domain/common/repo/pg"
	"github.com/rendau/kusec/internal/domain/item/model"
	repoModel "github.com/rendau/kusec/internal/domain/item/repo/db/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/infra/secretbox"
)

// keyUniqueIndex — уникальный индекс item(secret_id, key), см. migrations/000001_init.up.sql.
const keyUniqueIndex = "uq_item_secret_id_key"

// valueScope — контекст шифрования колонки item.value (уходит в AAD):
// шифртекст, переставленный в другую колонку, не расшифруется.
const valueScope = "item.value"

type Repo struct {
	*commonRepoPg.Base
	ModelStore *mobone.ModelStore

	// box шифрует value при записи и расшифровывает при чтении: всё, что выше
	// репозитория, работает с открытым значением. Без ключа — проходит как есть.
	box *secretbox.Box
}

func New(con *pgxpool.Pool, box *secretbox.Box) *Repo {
	base := commonRepoPg.NewBase(con)
	return &Repo{
		Base: base,
		ModelStore: &mobone.ModelStore{
			Con:                base.Con,
			QB:                 base.QB,
			TransactionManager: base.TxM,
			TableName:          "item",
		},
		box: box,
	}
}

func (r *Repo) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	// по шифртексту SQL искать не может: поиск по значению — в памяти
	if r.searchValueInMemory(pars) {
		return r.listSearchByValue(ctx, pars)
	}

	conditions, conditionExps := r.getConditions(pars, true)
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
	for _, item := range items {
		if err = r.openValue(item); err != nil {
			return nil, 0, err
		}
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
	if err = r.openValue(m); err != nil {
		return nil, false, err
	}
	return repoModel.EncodeSelect(m, 0), true, nil
}

func (r *Repo) Create(ctx context.Context, obj *model.Edit) (string, error) {
	m := repoModel.DecodeUpsert(obj)
	if err := r.sealValue(m); err != nil {
		return "", err
	}
	if err := r.ModelStore.Create(ctx, m); err != nil {
		if commonRepoPg.IsUniqueViolation(err, keyUniqueIndex) {
			return "", commonRepoPg.ExistsErr(errs.ItemKeyExists, "item key", obj.Key, "in the secret")
		}
		return "", fmt.Errorf("ModelStore.Create: %w", err)
	}
	return m.NewId, nil
}

func (r *Repo) Update(ctx context.Context, id string, obj *model.Edit) error {
	m := repoModel.DecodeUpsert(obj)
	m.PKId = id
	if err := r.sealValue(m); err != nil {
		return err
	}
	if err := r.ModelStore.Update(ctx, m); err != nil {
		if commonRepoPg.IsUniqueViolation(err, keyUniqueIndex) {
			return commonRepoPg.ExistsErr(errs.ItemKeyExists, "item key", obj.Key, "in the secret")
		}
		return fmt.Errorf("ModelStore.Update: %w", err)
	}
	return nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	m := &repoModel.Upsert{PKId: id}
	if err := r.ModelStore.Delete(ctx, m); err != nil {
		return fmt.Errorf("ModelStore.Delete: %w", err)
	}
	return nil
}

// sealValue шифрует значение перед записью. Указатель в m подменяется новым:
// m.Value указывает на поле доменной модели вызывающего, её не трогаем.
func (r *Repo) sealValue(m *repoModel.Upsert) error {
	if m.Value == nil {
		return nil
	}
	sealed, err := r.box.Seal(valueScope, *m.Value)
	if err != nil {
		if errors.Is(err, secretbox.ErrReservedPrefix) {
			return errs.ErrFull{
				Err:  errs.InvalidRequest,
				Desc: `item value must not start with "` + secretbox.Prefix + `"`,
			}
		}
		return fmt.Errorf("seal item value: %w", err)
	}
	m.Value = &sealed
	return nil
}

// openValue расшифровывает прочитанное значение на месте. Ошибку не глотаем:
// иначе наверх (в ответ API, в sync) ушёл бы шифртекст или пустота вместо
// значения. В текст ошибки попадает только id.
func (r *Repo) openValue(m *repoModel.Select) error {
	plain, err := r.box.Open(valueScope, m.Value)
	if err != nil {
		return fmt.Errorf("open item %s value: %w", m.Id, err)
	}
	m.Value = plain
	return nil
}
