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
	Active      bool
	Namespace   string
	Name        string
	SlugName    string
	Description string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	UpdatedAt   *time.Time
	Active      *bool
	Namespace   *string
	Name        *string
	SlugName    *string
	Description *string
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	Active    *bool
	Namespace *string
	Search    *string
}
