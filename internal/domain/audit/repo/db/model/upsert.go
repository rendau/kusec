package model

import (
	"fmt"

	domainModel "github.com/rendau/kusec/internal/domain/audit/model"
)

// Upsert — только создание: записи аудита не редактируются.
type Upsert struct {
	NewId int64

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

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{
		"actor_name":  m.ActorName,
		"source":      m.Source,
		"request_id":  m.RequestId,
		"entity_type": m.EntityType,
		"entity_id":   m.EntityId,
		"namespace":   m.Namespace,
		"app_slug":    m.AppSlug,
		"kube_kind":   m.KubeKind,
		"kube_name":   m.KubeName,
		"key":         m.Key,
		"action":      m.Action,
		"changes":     m.Changes,
	}
	if m.ActorUsrId != nil {
		result["actor_usr_id"] = *m.ActorUsrId
	}
	if m.ActorApiKeyId != nil {
		result["actor_api_key_id"] = *m.ActorApiKeyId
	}
	if m.AppId != nil {
		result["app_id"] = *m.AppId
	}
	if m.BatchId != nil {
		result["batch_id"] = *m.BatchId
	}
	return result
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.NewId}
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.NewId}
}

// DTO

func DecodeUpsert(v *domainModel.Main) (*Upsert, error) {
	changes, err := encodeChanges(v.Changes)
	if err != nil {
		return nil, fmt.Errorf("encodeChanges: %w", err)
	}

	return &Upsert{
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
		Changes:       changes,
		BatchId:       v.BatchId,
	}, nil
}
