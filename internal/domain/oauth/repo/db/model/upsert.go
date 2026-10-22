package model

import (
	domainModel "github.com/rendau/kusec/internal/domain/oauth/model"
)

type GrantUpsert struct {
	PKId string

	UsrId         int64
	ClientId      string
	ClientName    string
	RedirectUri   string
	CodeChallenge string
}

func (m *GrantUpsert) CreateColumnMap() map[string]any {
	return map[string]any{
		"usr_id":         m.UsrId,
		"client_id":      m.ClientId,
		"client_name":    m.ClientName,
		"redirect_uri":   m.RedirectUri,
		"code_challenge": m.CodeChallenge,
	}
}

func (m *GrantUpsert) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.PKId}
}

func (m *GrantUpsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func DecodeGrantUpsert(v *domainModel.Grant) *GrantUpsert {
	return &GrantUpsert{
		UsrId:         v.UsrId,
		ClientId:      v.ClientId,
		ClientName:    v.ClientName,
		RedirectUri:   v.RedirectUri,
		CodeChallenge: v.CodeChallenge,
	}
}
