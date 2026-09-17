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

	// Вычисляемые поля (не хранятся в базе, заполняются в usecase):
	// размер значения в байтах и его HMAC-отпечаток. Для read_only-сессий
	// Value обнуляется — остаются только эти поля.
	ValueSize int64
	ValueHash string
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
	AppId        *string
	Active       *bool
	Search       *string
	UpdatedAtGte *time.Time
	UpdatedAtLt  *time.Time
}
