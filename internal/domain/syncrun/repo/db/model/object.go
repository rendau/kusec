package model

import (
	domainModel "github.com/rendau/kusec/internal/domain/syncrun/model"
)

// ObjectSelect — строка sync_run_object.
type ObjectSelect struct {
	Id          int64
	RunId       string
	Namespace   string
	KubeKind    string
	KubeName    string
	Op          string
	Error       string
	ContentHash string
	ChangedKeys []string
}

func (m *ObjectSelect) ListColumnMap() map[string]any {
	return map[string]any{
		"id":           &m.Id,
		"run_id":       &m.RunId,
		"namespace":    &m.Namespace,
		"kube_kind":    &m.KubeKind,
		"kube_name":    &m.KubeName,
		"op":           &m.Op,
		"error":        &m.Error,
		"content_hash": &m.ContentHash,
		"changed_keys": &m.ChangedKeys,
	}
}

func (m *ObjectSelect) PKColumnMap() map[string]any {
	return map[string]any{"id": m.Id}
}

func (m *ObjectSelect) DefaultSortColumns() []string {
	return []string{"namespace", "kube_name"}
}

// ObjectUpsert — только создание: журнал не редактируется.
type ObjectUpsert struct {
	NewId int64

	RunId       string
	Namespace   string
	KubeKind    string
	KubeName    string
	Op          string
	Error       string
	ContentHash string
	ChangedKeys []string
}

func (m *ObjectUpsert) CreateColumnMap() map[string]any {
	changedKeys := m.ChangedKeys
	if changedKeys == nil {
		changedKeys = []string{}
	}
	return map[string]any{
		"run_id":       m.RunId,
		"namespace":    m.Namespace,
		"kube_kind":    m.KubeKind,
		"kube_name":    m.KubeName,
		"op":           m.Op,
		"error":        m.Error,
		"content_hash": m.ContentHash,
		"changed_keys": changedKeys,
	}
}

func (m *ObjectUpsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.NewId}
}

func (m *ObjectUpsert) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.NewId}
}

// DTO

func EncodeObjectSelect(v *ObjectSelect, _ int) *domainModel.Object {
	return &domainModel.Object{
		Id:          v.Id,
		RunId:       v.RunId,
		Namespace:   v.Namespace,
		KubeKind:    v.KubeKind,
		KubeName:    v.KubeName,
		Op:          v.Op,
		Error:       v.Error,
		ContentHash: v.ContentHash,
		ChangedKeys: v.ChangedKeys,
	}
}

func DecodeObjectUpsert(v *domainModel.Object) *ObjectUpsert {
	return &ObjectUpsert{
		RunId:       v.RunId,
		Namespace:   v.Namespace,
		KubeKind:    v.KubeKind,
		KubeName:    v.KubeName,
		Op:          v.Op,
		Error:       v.Error,
		ContentHash: v.ContentHash,
		ChangedKeys: v.ChangedKeys,
	}
}
