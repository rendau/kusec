package model

import (
	"time"

	commonModel "github.com/rendau/kusec/internal/domain/common/model"
)

// Статусы запуска sync.
const (
	StatusRunning = "running"
	StatusOk      = "ok"
	StatusPartial = "partial"
	StatusError   = "error"
)

// Операции над k8s-объектом в рамках запуска sync.
const (
	OpCreated   = "created"
	OpUpdated   = "updated"
	OpDeleted   = "deleted"
	OpUnchanged = "unchanged"
	OpError     = "error"
)

// Main — запуск синхронизации kusec → Kubernetes.
type Main struct {
	Id         string
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     string
	Error      string

	ActorUsrId    *int64
	ActorApiKeyId *string
	ActorName     string
	Source        string
	RequestId     string

	// AppId nil — sync всех доступных app.
	AppId      *string
	DurationMs int64

	// Objects — затронутые k8s-объекты; в sync_run не хранятся,
	// заполняются отдельно (Get).
	Objects []*Object
}

// Object — один k8s-объект запуска sync. Значения ключей не сохраняются:
// только имена изменившихся ключей и HMAC-отпечаток содержимого.
type Object struct {
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

// Edit — финализация запуска (единственная разрешённая мутация).
type Edit struct {
	FinishedAt *time.Time
	Status     *string
	Error      *string
	DurationMs *int64
}

// ListReq — параметры выборки запусков
type ListReq struct {
	commonModel.ListParams

	AppId *string
	// AppIds — ограничение доступа сессии: показываются запуски этих app
	// и глобальные (app_id is null).
	AppIds       []string
	Namespace    *string
	Status       *string
	StartedAtGte *time.Time
	StartedAtLt  *time.Time
}
