package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/item/model"
)

type Select struct {
	Id          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AppId       string
	Active      bool
	Key         string
	Value       string
	Description string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":          &m.Id,
		"created_at":  &m.CreatedAt,
		"updated_at":  &m.UpdatedAt,
		"app_id":      &m.AppId,
		"active":      &m.Active,
		"key":         &m.Key,
		"value":       &m.Value,
		"description": &m.Description,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"key"}
}

// DTO

func EncodeSelect(v *Select, _ int) *domainModel.Main {
	return &domainModel.Main{
		Id:          v.Id,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		AppId:       v.AppId,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		Description: v.Description,
	}
}
