package db

import (
	"github.com/rendau/kusec/internal/domain/apikey/model"
)

var allowedSortFields = map[string]string{
	"id":           "id",
	"created_at":   "created_at",
	"updated_at":   "updated_at",
	"name":         "name",
	"last_used_at": "last_used_at",
}

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any, 10)
	conditionExps := make(map[string][]any, 10)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.UsrId != nil {
		conditions["usr_id"] = *pars.UsrId
	}
	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}
	if pars.KeyHash != nil {
		conditions["key_hash"] = *pars.KeyHash
	}

	return conditions, conditionExps
}
