package model

import (
	domainModel "github.com/rendau/kusec/internal/domain/usr/model"
)

type Upsert struct {
	PKId  int64
	NewId int64

	Active      *bool
	IsAdmin     *bool
	Name        *string
	Username    *string
	Password    *string
	TotpSecret  *string
	TotpEnabled *bool
	AppIds      []string
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any, 10)
	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.IsAdmin != nil {
		result["is_admin"] = *m.IsAdmin
	}
	if m.Name != nil {
		result["name"] = *m.Name
	}
	if m.Username != nil {
		result["username"] = *m.Username
	}
	if m.Password != nil {
		result["password"] = *m.Password
	}
	if m.TotpSecret != nil {
		result["totp_secret"] = *m.TotpSecret
	}
	if m.TotpEnabled != nil {
		result["totp_enabled"] = *m.TotpEnabled
	}
	if m.AppIds != nil {
		result["app_ids"] = m.AppIds
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
		Active:      v.Active,
		IsAdmin:     v.IsAdmin,
		Name:        v.Name,
		Username:    v.Username,
		Password:    v.Password,
		TotpSecret:  v.TotpSecret,
		TotpEnabled: v.TotpEnabled,
		AppIds:      v.AppIds,
	}
}
