package model

import (
	"time"

	domainModel "github.com/rendau/kusec/internal/domain/syncrun/model"
)

// Upsert — создание запуска (actor-поля) и финализация (status/finished_at/
// error/duration_ms).
type Upsert struct {
	PKId  string
	NewId string

	Status        *string
	Error         *string
	FinishedAt    *time.Time
	ActorUsrId    *int64
	ActorApiKeyId *string
	ActorName     *string
	Source        *string
	RequestId     *string
	AppId         *string
	DurationMs    *int64
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := make(map[string]any, 10)
	if m.Status != nil {
		result["status"] = *m.Status
	}
	if m.Error != nil {
		result["error"] = *m.Error
	}
	if m.FinishedAt != nil {
		result["finished_at"] = *m.FinishedAt
	}
	if m.ActorUsrId != nil {
		result["actor_usr_id"] = *m.ActorUsrId
	}
	if m.ActorApiKeyId != nil {
		result["actor_api_key_id"] = *m.ActorApiKeyId
	}
	if m.ActorName != nil {
		result["actor_name"] = *m.ActorName
	}
	if m.Source != nil {
		result["source"] = *m.Source
	}
	if m.RequestId != nil {
		result["request_id"] = *m.RequestId
	}
	if m.AppId != nil {
		result["app_id"] = *m.AppId
	}
	if m.DurationMs != nil {
		result["duration_ms"] = *m.DurationMs
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

// DecodeCreate — поля нового запуска (актор и область).
func DecodeCreate(v *domainModel.Main) *Upsert {
	return &Upsert{
		Status:        &v.Status,
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     &v.ActorName,
		Source:        &v.Source,
		RequestId:     &v.RequestId,
		AppId:         v.AppId,
	}
}

// DecodeEdit — финализация запуска.
func DecodeEdit(v *domainModel.Edit) *Upsert {
	return &Upsert{
		Status:     v.Status,
		Error:      v.Error,
		FinishedAt: v.FinishedAt,
		DurationMs: v.DurationMs,
	}
}
