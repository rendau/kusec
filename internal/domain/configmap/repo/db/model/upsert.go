package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/configmap/model"
)

type Upsert struct {
	PKId  string
	NewId string

	UpdatedAt   *time.Time
	AppId       *string
	Active      *bool
	SlugName    *string
	Description *string
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any, 10)
	if m.UpdatedAt != nil {
		result["updated_at"] = *m.UpdatedAt
	}
	if m.AppId != nil {
		result["app_id"] = *m.AppId
	}
	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.SlugName != nil {
		result["slug_name"] = *m.SlugName
	}
	if m.Description != nil {
		result["description"] = *m.Description
	}
	return result
}

func (m *Upsert) UpdateColumnMap() map[string]any {
	return m.CreateColumnMap()
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.NewId}
}

// DTO

func DecodeUpsert(v *domainModel.Edit) *Upsert {
	return &Upsert{
		UpdatedAt:   v.UpdatedAt,
		AppId:       v.AppId,
		Active:      v.Active,
		SlugName:    v.SlugName,
		Description: v.Description,
	}
}
