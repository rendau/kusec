package db

import (
	"github.com/rendau/kusec/internal/domain/item/model"
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"created_at": "created_at",
	"updated_at": "updated_at",
	"key":        "key",
}

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any, 10)
	conditionExps := make(map[string][]any, 10)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.SecretId != nil {
		conditions["secret_id"] = *pars.SecretId
	}
	if len(pars.SecretIds) > 0 {
		conditions["secret_id"] = pars.SecretIds
	}
	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}
	if pars.Search != nil {
		conditionExps["(key ILIKE ? OR value ILIKE ? OR description ILIKE ?)"] = []any{
			"%" + *pars.Search + "%",
			"%" + *pars.Search + "%",
			"%" + *pars.Search + "%",
		}
	}

	return conditions, conditionExps
}
