package model

import (
	"time"

	commonModel "github.com/mechta-market/kusec/internal/domain/common/model"
)

// Main — доменная сущность (все поля value-типы)
type Main struct {
	Id          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SecretId    string
	Active      bool
	Key         string
	Value       string
	ValueFormat string
	Description string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	UpdatedAt   *time.Time
	SecretId    *string
	Active      *bool
	Key         *string
	Value       *string
	ValueFormat *string
	Description *string
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	SecretId *string
	Active   *bool
	Search   *string
}
