package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
)

type Upsert struct {
	PKId  string
	NewId string

	UpdatedAt   *time.Time
	ConfigMapId *string
	Active      *bool
	Key         *string
	Value       *string
	ValueFormat *string
	Encoding    *string
	FileName    *string
	ContentType *string
	Description *string
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any, 10)
	if m.UpdatedAt != nil {
		result["updated_at"] = *m.UpdatedAt
	}
	if m.ConfigMapId != nil {
		result["configmap_id"] = *m.ConfigMapId
	}
	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.Key != nil {
		result["key"] = *m.Key
	}
	if m.Value != nil {
		result["value"] = *m.Value
	}
	if m.ValueFormat != nil {
		result["value_format"] = *m.ValueFormat
	}
	if m.Encoding != nil {
		result["encoding"] = *m.Encoding
	}
	if m.FileName != nil {
		result["file_name"] = *m.FileName
	}
	if m.ContentType != nil {
		result["content_type"] = *m.ContentType
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
		ConfigMapId: v.ConfigMapId,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		ValueFormat: v.ValueFormat,
		Encoding:    v.Encoding,
		FileName:    v.FileName,
		ContentType: v.ContentType,
		Description: v.Description,
	}
}
