package db

import (
	"github.com/mechta-market/kusec/internal/domain/configitem/model"
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

	if pars.ConfigMapId != nil {
		conditions["configmap_id"] = *pars.ConfigMapId
	}
	if len(pars.ConfigMapIds) > 0 {
		conditions["configmap_id"] = pars.ConfigMapIds
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
