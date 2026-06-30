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
	ConfigMapId string
	Active      bool
	Key         string
	Value       string
	ValueFormat string
	Encoding    string
	FileName    string
	ContentType string
	Description string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	UpdatedAt   *time.Time
	ConfigMapId *string
	Active      *bool
	Key         *string
	Value       *string
	ValueFormat *string
	Encoding    *string
	FileName    *string
	ContentType *string
	Description *string
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	ConfigMapId  *string
	ConfigMapIds []string
	Active       *bool
	Search       *string
}
