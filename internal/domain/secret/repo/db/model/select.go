package model

import (
	"time"

	domainModel "github.com/rendau/kusec/internal/domain/secret/model"
)

type Select struct {
	Id          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AppId       string
	Active      bool
	SlugName    string
	Description string
	KubeType    string
	ExactSlug   bool
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":          &m.Id,
		"created_at":  &m.CreatedAt,
		"updated_at":  &m.UpdatedAt,
		"app_id":      &m.AppId,
		"active":      &m.Active,
		"slug_name":   &m.SlugName,
		"description": &m.Description,
		"kube_type":   &m.KubeType,
		"exact_slug":  &m.ExactSlug,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"slug_name"}
}

// DTO

func EncodeSelect(v *Select, _ int) *domainModel.Main {
	return &domainModel.Main{
		Id:          v.Id,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		AppId:       v.AppId,
		Active:      v.Active,
		SlugName:    v.SlugName,
		Description: v.Description,
		KubeType:    v.KubeType,
		ExactSlug:   v.ExactSlug,
	}
}
