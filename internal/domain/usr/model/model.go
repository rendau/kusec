package model

import (
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
)

// Main — доменная сущность (все поля value-типы)
type Main struct {
	Id       int64
	Active   bool
	IsAdmin  bool
	Name     string
	Username string
	Password string // хеш пароля; наружу (в proto) не отдаётся — usecase обнуляет перед ответом

	TotpEnabled bool
	TotpSecret  string // секрет TOTP (base32); наружу не отдаётся

	AppIds []string
}

// Edit — мутация (все поля pointer-типы для partial update)
type Edit struct {
	Active   *bool
	IsAdmin  *bool
	Name     *string
	Username *string
	Password *string

	TotpEnabled *bool
	TotpSecret  *string

	AppIds []string
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	Active  *bool
	IsAdmin *bool
	Search  *string
}
