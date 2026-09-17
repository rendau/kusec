package db

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/kusec/internal/domain/audit/model"
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"created_at": "created_at",
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
		conditions["app_id"] = pars.AppIds
	}
	if pars.Namespace != nil {
		conditions["namespace"] = *pars.Namespace
	}
	if pars.AppSlug != nil {
		conditions["app_slug"] = *pars.AppSlug
	}
	if pars.KubeName != nil {
		conditions["kube_name"] = *pars.KubeName
	}
	if pars.EntityType != nil {
		conditions["entity_type"] = *pars.EntityType
	}
	if pars.EntityId != nil {
		conditions["entity_id"] = *pars.EntityId
	}
	if pars.Action != nil {
		conditions["action"] = *pars.Action
	}
	if pars.ActorUsrId != nil {
		conditions["actor_usr_id"] = *pars.ActorUsrId
	}
	if pars.Key != nil {
		conditions["key"] = *pars.Key
	}
	if pars.BatchId != nil {
		conditions["batch_id"] = *pars.BatchId
	}
	if pars.CreatedAtGte != nil {
		conditionExps["created_at >= ?"] = []any{*pars.CreatedAtGte}
	}
	if pars.CreatedAtLt != nil {
		conditionExps["created_at < ?"] = []any{*pars.CreatedAtLt}
	}

	return conditions, conditionExps
}

// LastEditors возвращает имя последнего актора по каждому item/config_item
// приложения (для updated_by в /app/{id}/keys). Ключ карты — entity_id.
func (r *Repo) LastEditors(ctx context.Context, appId string) (map[string]string, error) {
	rows, err := r.TxM.GetConnection(ctx).Query(ctx, `
		select distinct on (entity_id) entity_id, actor_name
		from audit
		where app_id = $1 and entity_type in ('item', 'config_item')
		order by entity_id, created_at desc
	`, appId)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	result := map[string]string{}
	for rows.Next() {
		var entityId, actorName string
		if err = rows.Scan(&entityId, &actorName); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		result[entityId] = actorName
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return result, nil
}

// DeleteOlderThan удаляет записи старше границы (фоновый ретеншн).
func (r *Repo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.TxM.GetConnection(ctx).Exec(ctx, `delete from audit where created_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("exec: %w", err)
	}
	return tag.RowsAffected(), nil
}
