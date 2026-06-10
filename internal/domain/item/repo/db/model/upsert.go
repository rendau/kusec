package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/item/model"
)

type Upsert struct {
	PKId  string
	NewId string

	UpdatedAt   *time.Time
	SecretId    *string
	Active      *bool
	Key         *string
	Value       *string
	ValueFormat *string
	Description *string
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any, 10)
	if m.UpdatedAt != nil {
		result["updated_at"] = *m.UpdatedAt
	}
	if m.SecretId != nil {
		result["secret_id"] = *m.SecretId
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
		SecretId:    v.SecretId,
		Active:      v.Active,
		Key:         v.Key,
		Value:       v.Value,
		ValueFormat: v.ValueFormat,
		Description: v.Description,
	}
}
