package model

import (
	"time"

	domainModel "github.com/rendau/kusec/internal/domain/apikey/model"
)

type Select struct {
	Id         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	UsrId      int64
	Active     bool
	McpOnly    bool
	Name       string
	KeyHash    string
	KeyPrefix  string
	LastUsedAt *time.Time
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":           &m.Id,
		"created_at":   &m.CreatedAt,
		"updated_at":   &m.UpdatedAt,
		"usr_id":       &m.UsrId,
		"active":       &m.Active,
		"mcp_only":     &m.McpOnly,
		"name":         &m.Name,
		"key_hash":     &m.KeyHash,
		"key_prefix":   &m.KeyPrefix,
		"last_used_at": &m.LastUsedAt,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"created_at"}
}

// DTO

func EncodeSelect(v *Select, _ int) *domainModel.Main {
	return &domainModel.Main{
		Id:         v.Id,
		CreatedAt:  v.CreatedAt,
		UpdatedAt:  v.UpdatedAt,
		UsrId:      v.UsrId,
		Active:     v.Active,
		McpOnly:    v.McpOnly,
		Name:       v.Name,
		KeyHash:    v.KeyHash,
		KeyPrefix:  v.KeyPrefix,
		LastUsedAt: v.LastUsedAt,
	}
}
