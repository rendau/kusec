package model

import (
	"time"

	commonModel "github.com/mechta-market/kusec/internal/domain/common/model"
)

// Main — доменная сущность (все поля value-типы).
// KeyHash — sha256-хэш ключа; сам ключ не хранится и наружу не отдаётся.
type Main struct {
	Id         string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	UsrId      int64
	Active     bool
	Name       string
	KeyHash    string
	KeyPrefix  string
	LastUsedAt *time.Time
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	UpdatedAt  *time.Time
	UsrId      *int64
	Active     *bool
	Name       *string
	KeyHash    *string
	KeyPrefix  *string
	LastUsedAt *time.Time
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	UsrId   *int64
	Active  *bool
	KeyHash *string
}
