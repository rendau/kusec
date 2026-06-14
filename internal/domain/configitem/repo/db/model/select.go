package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/configitem/model"
)

type Select struct {
	Id          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ConfigMapId string
	Active      bool
	Key         string
	Value       string
	ValueFormat string
	Encoding    string
	FileName    string
	ContentType string
	Description string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":           &m.Id,
		"created_at":   &m.CreatedAt,
		"updated_at":   &m.UpdatedAt,
		"configmap_id": &m.ConfigMapId,
		"active":       &m.Active,
		"key":          &m.Key,
		"value":        &m.Value,
		"value_format": &m.ValueFormat,
		"encoding":     &m.Encoding,
		"file_name":    &m.FileName,
		"content_type": &m.ContentType,
		"description":  &m.Description,
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
