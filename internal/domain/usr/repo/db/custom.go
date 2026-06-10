package db

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/mechta-market/kusec/internal/domain/usr/model"
	repoModel "github.com/mechta-market/kusec/internal/domain/usr/repo/db/model"
)

var allowedSortFields = map[string]string{
	"id":       "id",
	"name":     "name",
	"username": "username",
}

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any, 10)
	conditionExps := make(map[string][]any, 10)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}
	if pars.IsAdmin != nil {
		conditions["is_admin"] = *pars.IsAdmin
	}
	if pars.Search != nil {
		conditionExps["(name ILIKE ? OR username ILIKE ?)"] = []any{"%" + *pars.Search + "%", "%" + *pars.Search + "%"}
	}

	return conditions, conditionExps
}

func (r *Repo) GetByUsername(ctx context.Context, username string) (*model.Main, bool, error) {
	m := &repoModel.Select{}

	colMap := m.ListColumnMap()
	colNames := make([]string, 0, len(colMap))
	colPointers := make([]any, 0, len(colMap))
	for name, ptr := range colMap {
		colNames = append(colNames, name)
		colPointers = append(colPointers, ptr)
	}

	query, args, err := r.QB.
		Select(colNames...).
		From("usr").
		Where(sq.Eq{"username": username}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("GetByUsername build query: %w", err)
	}

	err = r.Con.QueryRow(ctx, query, args...).Scan(colPointers...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("GetByUsername: %w", err)
	}

	return repoModel.EncodeSelect(m, 0), true, nil
}

func (r *Repo) HasAny(ctx context.Context) (bool, error) {
	query, args, err := r.QB.
		Select("1").
		From("usr").
		Limit(1).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("HasAny build query: %w", err)
	}

	var exists int
	err = r.Con.QueryRow(ctx, query, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("HasAny: %w", err)
	}

	return true, nil
}
