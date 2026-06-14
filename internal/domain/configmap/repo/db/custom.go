package db

import (
	"github.com/mechta-market/kusec/internal/domain/configmap/model"
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

	return conditions, conditionExps
}
