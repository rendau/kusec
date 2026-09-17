package model

import (
	"time"

	domainModel "github.com/rendau/kusec/internal/domain/audit/model"
)

type Select struct {
	Id            int64
	CreatedAt     time.Time
	ActorUsrId    *int64
	ActorApiKeyId *string
	ActorName     string
	Source        string
	RequestId     string
	EntityType    string
	EntityId      string
	AppId         *string
	Namespace     string
	AppSlug       string
	KubeKind      string
	KubeName      string
	Key           string
	Action        string
	Changes       []byte
	BatchId       *string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":               &m.Id,
		"created_at":       &m.CreatedAt,
		"actor_usr_id":     &m.ActorUsrId,
		"actor_api_key_id": &m.ActorApiKeyId,
		"actor_name":       &m.ActorName,
		"source":           &m.Source,
		"request_id":       &m.RequestId,
		"entity_type":      &m.EntityType,
		"entity_id":        &m.EntityId,
		"app_id":           &m.AppId,
		"namespace":        &m.Namespace,
		"app_slug":         &m.AppSlug,
		"kube_kind":        &m.KubeKind,
		"kube_name":        &m.KubeName,
		"key":              &m.Key,
		"action":           &m.Action,
		"changes":          &m.Changes,
		"batch_id":         &m.BatchId,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"created_at desc", "id desc"}
}

// DTO

func EncodeSelect(v *Select, _ int) *domainModel.Main {
	return &domainModel.Main{
		Id:            v.Id,
		CreatedAt:     v.CreatedAt,
		ActorUsrId:    v.ActorUsrId,
		ActorApiKeyId: v.ActorApiKeyId,
		ActorName:     v.ActorName,
		Source:        v.Source,
		RequestId:     v.RequestId,
		EntityType:    v.EntityType,
		EntityId:      v.EntityId,
		AppId:         v.AppId,
		Namespace:     v.Namespace,
		AppSlug:       v.AppSlug,
		KubeKind:      v.KubeKind,
		KubeName:      v.KubeName,
		Key:           v.Key,
		Action:        v.Action,
		Changes:       decodeChanges(v.Changes),
		BatchId:       v.BatchId,
	}
}
