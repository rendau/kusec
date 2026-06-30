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
	// При true имя k8s-configmap = SlugName без префикса и app-slug.
	// Менять флаг могут только админы.
	ExactSlug bool

	// Вычисляемое поле: не хранится в базе, заполняется в usecase.
	KubeConfigMapName string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	UpdatedAt   *time.Time
	AppId       *string
	Active      *bool
	SlugName    *string
	Description *string
	ExactSlug   *bool
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	Ids    []string
	AppId  *string
	AppIds []string
	Active *bool
	Search *string
}
