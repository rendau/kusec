package model

import (
	"time"

	commonModel "github.com/rendau/kusec/internal/domain/common/model"
)

// Main — доменная сущность (все поля value-типы)
type Main struct {
	Id          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AppId       string
	Active      bool
	SlugName    string
	Description string
	// Тип k8s-секрета (пусто = Opaque), например kubernetes.io/basic-auth.
	KubeType string
	// При true имя k8s-секрета = SlugName без префикса и app-slug.
	// Менять флаг могут только админы.
	ExactSlug bool

	// Последний применённый в кластер снимок (обновляется sync-ом, минуя
	// updated_at): nil — ещё ни разу не синхронизирован.
	LastSyncedAt   *time.Time
	LastSyncedHash string

	// Вычисляемое поле: не хранится в базе, заполняется в usecase.
	KubeSecretName string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	UpdatedAt   *time.Time
	AppId       *string
	Active      *bool
	SlugName    *string
	Description *string
	KubeType    *string
	ExactSlug   *bool
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	Ids          []string
	AppId        *string
	AppIds       []string
	Active       *bool
	Search       *string
	UpdatedAtGte *time.Time
	UpdatedAtLt  *time.Time
}
