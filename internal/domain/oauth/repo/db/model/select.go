package model

import (
	"time"

	domainModel "github.com/rendau/kusec/internal/domain/oauth/model"
)

type GrantSelect struct {
	PKId string

	UsrId         int64
	ClientId      string
	ClientName    string
	RedirectUri   string
	CodeChallenge string
	CreatedAt     time.Time
	ConnectedAt   *time.Time
	LastUsedAt    *time.Time
}

func (m *GrantSelect) ListColumnMap() map[string]any {
	return map[string]any{
		"id":             &m.PKId,
		"usr_id":         &m.UsrId,
		"client_id":      &m.ClientId,
		"client_name":    &m.ClientName,
		"redirect_uri":   &m.RedirectUri,
		"code_challenge": &m.CodeChallenge,
		"created_at":     &m.CreatedAt,
		"connected_at":   &m.ConnectedAt,
		"last_used_at":   &m.LastUsedAt,
	}
}

func (m *GrantSelect) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *GrantSelect) DefaultSortColumns() []string {
	return []string{"created_at", "id"}
}

func EncodeGrantSelect(v *GrantSelect, _ int) *domainModel.Grant {
	if v == nil {
		return nil
	}
	return &domainModel.Grant{
		Id:            v.PKId,
		UsrId:         v.UsrId,
		ClientId:      v.ClientId,
		ClientName:    v.ClientName,
		RedirectUri:   v.RedirectUri,
		CodeChallenge: v.CodeChallenge,
		CreatedAt:     v.CreatedAt,
		ConnectedAt:   v.ConnectedAt,
		LastUsedAt:    v.LastUsedAt,
	}
}
