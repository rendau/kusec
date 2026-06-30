package db

import (
	"github.com/rendau/kusec/internal/domain/app/model"
)

var allowedSortFields = map[string]string{
	"id":         "id",
	"created_at": "created_at",
	"updated_at": "updated_at",
	"name":       "name",
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
	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}
	if pars.Namespace != nil {
		conditions["namespace"] = *pars.Namespace
	}
	if pars.Search != nil {
		conditionExps["(name ILIKE ? OR slug_name ILIKE ? OR description ILIKE ?)"] = []any{"%" + *pars.Search + "%", "%" + *pars.Search + "%", "%" + *pars.Search + "%"}
	}

	return conditions, conditionExps
}
