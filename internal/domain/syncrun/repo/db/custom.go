package db

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/kusec/internal/domain/syncrun/model"
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"started_at": "started_at",
	"status":     "status",
}

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any, 10)
	conditionExps := make(map[string][]any, 10)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.AppId != nil {
		conditions["app_id"] = *pars.AppId
	}
	if len(pars.AppIds) > 0 {
		// доступ сессии: запуски своих app + глобальные запуски
		conditionExps["(app_id = any(?) or app_id is null)"] = []any{pars.AppIds}
	}
	if pars.Namespace != nil {
		conditionExps["id IN (SELECT run_id FROM sync_run_object WHERE namespace = ?)"] = []any{*pars.Namespace}
	}
	if pars.Status != nil {
		conditions["status"] = *pars.Status
	}
	if pars.StartedAtGte != nil {
		conditionExps["started_at >= ?"] = []any{*pars.StartedAtGte}
	}
	if pars.StartedAtLt != nil {
		conditionExps["started_at < ?"] = []any{*pars.StartedAtLt}
	}

	return conditions, conditionExps
}

// DeleteOlderThan удаляет запуски старше границы (объекты — каскадом).
func (r *Repo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.TxM.GetConnection(ctx).Exec(ctx, `delete from sync_run where started_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
