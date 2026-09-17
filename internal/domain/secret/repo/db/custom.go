package db

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/kusec/internal/domain/secret/model"
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"created_at": "created_at",
	"updated_at": "updated_at",
	"slug_name":  "slug_name",
}

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any, 10)
	conditionExps := make(map[string][]any, 10)

	if pars == nil {
		return conditions, conditionExps
	}

	if len(pars.Ids) > 0 {
		conditions["id"] = pars.Ids
	}
	if pars.AppId != nil {
		conditions["app_id"] = *pars.AppId
	}
	if len(pars.AppIds) > 0 {
		conditions["app_id"] = pars.AppIds
	}
	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}
	if pars.Search != nil {
		conditionExps["(slug_name ILIKE ? OR description ILIKE ?)"] = []any{
			"%" + *pars.Search + "%",
			"%" + *pars.Search + "%",
		}
	}

	if pars.UpdatedAtGte != nil {
		conditionExps["updated_at >= ?"] = []any{*pars.UpdatedAtGte}
	}
	if pars.UpdatedAtLt != nil {
		conditionExps["updated_at < ?"] = []any{*pars.UpdatedAtLt}
	}

	return conditions, conditionExps
}

// TouchSynced отмечает последний применённый в кластер снимок записи.
// Пишется напрямую (мимо ModelStore.Update), чтобы не трогать updated_at:
// sync — не изменение конфигурации.
func (r *Repo) TouchSynced(ctx context.Context, id string, at time.Time, hash string) error {
	_, err := r.TxM.GetConnection(ctx).Exec(ctx,
		`update secret set last_synced_at = $2, last_synced_hash = $3 where id = $1`,
		id, at, hash)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}
