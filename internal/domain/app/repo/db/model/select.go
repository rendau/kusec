package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/app/model"
)

type Select struct {
	Id          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Active      bool
	Namespace   string
	Name        string
	Description string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":          &m.Id,
		"created_at":  &m.CreatedAt,
		"updated_at":  &m.UpdatedAt,
		"active":      &m.Active,
		"namespace":   &m.Namespace,
		"name":        &m.Name,
		"description": &m.Description,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"name"}
}

// DTO

func EncodeSelect(v *Select, _ int) *domainModel.Main {
	return &domainModel.Main{
		Id:          v.Id,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		Active:      v.Active,
		Namespace:   v.Namespace,
		Name:        v.Name,
		Description: v.Description,
	}
}
