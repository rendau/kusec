package model

import (
	commonModel "github.com/mechta-market/kusec/internal/domain/common/model"
)

// Main — доменная сущность (все поля value-типы)
type Main struct {
	Id       int64
	Active   bool
	IsAdmin  bool
	Name     string
	Username string
	Password string // хеш пароля; наружу (в proto) не отдаётся — usecase обнуляет перед ответом

	AppIds []string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	Active   *bool
	IsAdmin  *bool
	Name     *string
	Username *string
	Password *string

	AppIds []string
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	Active  *bool
	IsAdmin *bool
	Search  *string
}
