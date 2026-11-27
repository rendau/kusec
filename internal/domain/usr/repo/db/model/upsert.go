package model

import (
	sq "github.com/Masterminds/squirrel"

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
	AllApps     *bool
	AppIds      []string

	// SessionVersionInc — session_version = session_version + 1 (только Update).
	SessionVersionInc bool
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
	if m.AllApps != nil {
		result["all_apps"] = *m.AllApps
	}
	if m.AppIds != nil {
		result["app_ids"] = m.AppIds
	}
	return result
}

func (m *Upsert) UpdateColumnMap() map[string]any {
	result := m.CreateColumnMap()
	if m.SessionVersionInc {
		// тем же запросом, что и остальные поля: смена пароля и отзыв
		// токенов не расходятся даже вне транзакции
		result["session_version"] = sq.Expr("session_version + 1")
	}
	return result
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
		Active:            v.Active,
		IsAdmin:           v.IsAdmin,
		Name:              v.Name,
		Username:          v.Username,
		Password:          v.Password,
		TotpSecret:        v.TotpSecret,
		TotpEnabled:       v.TotpEnabled,
		AllApps:           v.AllApps,
		AppIds:            v.AppIds,
		SessionVersionInc: v.SessionVersionInc,
	}
}
