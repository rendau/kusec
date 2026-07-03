package model

import (
	"time"

	domainModel "github.com/mechta-market/kusec/internal/domain/apikey/model"
)

type Upsert struct {
	PKId  string
	NewId string

	UpdatedAt  *time.Time
	UsrId      *int64
	Active     *bool
	Name       *string
	KeyHash    *string
	KeyPrefix  *string
	LastUsedAt *time.Time
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any, 10)
	if m.UpdatedAt != nil {
		result["updated_at"] = *m.UpdatedAt
	}
	if m.UsrId != nil {
		result["usr_id"] = *m.UsrId
	}
	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.Name != nil {
		result["name"] = *m.Name
	}
	if m.KeyHash != nil {
		result["key_hash"] = *m.KeyHash
	}
	if m.KeyPrefix != nil {
		result["key_prefix"] = *m.KeyPrefix
	}
	if m.LastUsedAt != nil {
		result["last_used_at"] = *m.LastUsedAt
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
		UpdatedAt:  v.UpdatedAt,
		UsrId:      v.UsrId,
		Active:     v.Active,
		Name:       v.Name,
		KeyHash:    v.KeyHash,
		KeyPrefix:  v.KeyPrefix,
		LastUsedAt: v.LastUsedAt,
	}
}
