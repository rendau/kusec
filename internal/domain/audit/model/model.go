package model

import (
	"time"

	commonModel "github.com/rendau/kusec/internal/domain/common/model"
)

// Типы сущностей (audit.entity_type).
const (
	EntityApp        = "app"
	EntitySecret     = "secret"
	EntityItem       = "item"
	EntityConfigMap  = "configmap"
	EntityConfigItem = "config_item"
	EntityApiKey     = "api_key"
	EntityUsr        = "usr"
	EntitySyncRun    = "sync_run"
)

// Действия (audit.action).
const (
	ActionCreate     = "create"
	ActionUpdate     = "update"
	ActionDelete     = "delete"
	ActionActivate   = "activate"
	ActionDeactivate = "deactivate"
	ActionSync       = "sync"
	ActionImport     = "import"
)

// Виды k8s-объектов (audit.kube_kind).
const (
	KubeKindSecret    = "Secret"
	KubeKindConfigMap = "ConfigMap"
)

// Change — одно изменённое поле записи аудита.
// Для несекретных полей заполняются Old/New (nil — стороны нет: create/delete).
// Для значений item (secret) и усечённых больших значений — только
// HMAC-отпечаток и размер; само значение не сохраняется никогда.
type Change struct {
	Field   string
	Old     *string
	New     *string
	OldHash string
	NewHash string
	OldSize *int64
	NewSize *int64
	// Truncated — значение несекретное, но длиннее лимита: вместо old/new
	// сохранены отпечаток и размер.
	Truncated bool
}

// Main — запись аудита (insert-only: не редактируется и не удаляется через
// API, чистка — только фоновым ретеншном).
type Main struct {
	Id        int64
	CreatedAt time.Time

	// actor: имя денормализовано — пользователь или ключ могут быть удалены
	ActorUsrId    *int64
	ActorApiKeyId *string
	ActorName     string
	Source        string
	RequestId     string

	// объект
	EntityType string
	EntityId   string
	AppId      *string
	Namespace  string
	AppSlug    string
	KubeKind   string
	KubeName   string
	Key        string

	Action  string
	Changes []Change

	// BatchId группирует массовые операции (каскадное удаление, импорт, sync).
	BatchId *string
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	AppId      *string
	AppIds     []string // ограничение доступа сессии (не-админ)
	Namespace  *string
	AppSlug    *string
	KubeName   *string
	EntityType *string
	EntityId   *string
	Action     *string
	ActorUsrId *int64
	Key        *string
	BatchId    *string

	CreatedAtGte *time.Time
	CreatedAtLt  *time.Time
}
