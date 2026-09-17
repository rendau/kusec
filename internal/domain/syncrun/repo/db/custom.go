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

// AppSyncStats — сводка по каждому приложению: последнее применение в
// кластер и число secret/configmap с изменениями, ещё не применёнными
// (изменён после last_synced_at либо активен и ни разу не синхронизирован).
func (r *Repo) AppSyncStats(ctx context.Context) ([]*model.AppSyncStat, error) {
	rows, err := r.TxM.GetConnection(ctx).Query(ctx, `
		select a.namespace,
		       a.slug_name,
		       max(o.last_synced_at)                     as last_synced_at,
		       count(*) filter (where o.unsynced)::int8  as unsynced
		from app a
		join (
			select s.app_id,
			       s.last_synced_at,
			       (s.active and s.last_synced_at is null)
			        or (s.last_synced_at is not null
			            and greatest(s.updated_at, coalesce(i.max_updated, s.updated_at)) > s.last_synced_at) as unsynced
			from secret s
			left join lateral (
				select max(updated_at) as max_updated from item where secret_id = s.id
			) i on true
		  union all
			select c.app_id,
			       c.last_synced_at,
			       (c.active and c.last_synced_at is null)
			        or (c.last_synced_at is not null
			            and greatest(c.updated_at, coalesce(ci.max_updated, c.updated_at)) > c.last_synced_at) as unsynced
			from configmap c
			left join lateral (
				select max(updated_at) as max_updated from config_item where configmap_id = c.id
			) ci on true
		) o on o.app_id = a.id
		group by a.namespace, a.slug_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	result := make([]*model.AppSyncStat, 0)
	for rows.Next() {
		stat := &model.AppSyncStat{}
		if err = rows.Scan(&stat.Namespace, &stat.AppSlug, &stat.LastSyncedAt, &stat.UnsyncedCount); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result = append(result, stat)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return result, nil
}

// DeleteOlderThan удаляет запуски старше границы (объекты — каскадом).
func (r *Repo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.TxM.GetConnection(ctx).Exec(ctx, `delete from sync_run where started_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
