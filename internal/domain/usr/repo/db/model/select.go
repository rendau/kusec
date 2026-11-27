package model

import (
	domainModel "github.com/rendau/kusec/internal/domain/usr/model"
)

type Select struct {
	Id             int64
	Active         bool
	IsAdmin        bool
	Name           string
	Username       string
	Password       string
	TotpSecret     string
	TotpEnabled    bool
	AllApps        bool
	AppIds         []string
	SessionVersion int64
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":              &m.Id,
		"active":          &m.Active,
		"is_admin":        &m.IsAdmin,
		"name":            &m.Name,
		"username":        &m.Username,
		"password":        &m.Password,
		"totp_secret":     &m.TotpSecret,
		"totp_enabled":    &m.TotpEnabled,
		"all_apps":        &m.AllApps,
		"app_ids":         &m.AppIds,
		"session_version": &m.SessionVersion,
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
		Id:             v.Id,
		Active:         v.Active,
		IsAdmin:        v.IsAdmin,
		Name:           v.Name,
		Username:       v.Username,
		Password:       v.Password,
		TotpEnabled:    v.TotpEnabled,
		TotpSecret:     v.TotpSecret,
		AllApps:        v.AllApps,
		AppIds:         v.AppIds,
		SessionVersion: v.SessionVersion,
	}
}
