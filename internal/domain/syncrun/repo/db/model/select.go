package model

import (
	"time"

	domainModel "github.com/rendau/kusec/internal/domain/syncrun/model"
)

type Select struct {
	Id            string
	StartedAt     time.Time
	FinishedAt    *time.Time
	Status        string
	Error         string
	ActorUsrId    *int64
	ActorApiKeyId *string
	ActorName     string
	Source        string
	RequestId     string
	AppId         *string
	DurationMs    int64
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":               &m.Id,
		"started_at":       &m.StartedAt,
		"finished_at":      &m.FinishedAt,
		"status":           &m.Status,
		"error":            &m.Error,
		"actor_usr_id":     &m.ActorUsrId,
		"actor_api_key_id": &m.ActorApiKeyId,
		"actor_name":       &m.ActorName,
		"source":           &m.Source,
		"request_id":       &m.RequestId,
		"app_id":           &m.AppId,
		"duration_ms":      &m.DurationMs,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"started_at desc"}
}

// DTO

func EncodeSelect(v *Select, _ int) *domainModel.Main {
	return &domainModel.Main{
		Id:            v.Id,
		StartedAt:     v.StartedAt,
		FinishedAt:    v.FinishedAt,
		Status:        v.Status,
		Error:         v.Error,
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     v.ActorName,
		Source:        v.Source,
		RequestId:     v.RequestId,
		AppId:         v.AppId,
		DurationMs:    v.DurationMs,
	}
}
