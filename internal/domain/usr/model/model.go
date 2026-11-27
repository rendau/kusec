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

	// AllApps — не-админ видит все приложения. Иначе доступ только к AppIds,
	// пустой список = не видит ничего. У админа доступ ко всему всегда.
	AllApps bool
	AppIds  []string

	// SessionVersion — номер поколения сессий: зашит в токены (claim ver),
	// токен с другим номером не принимается. Наружу не отдаётся.
	SessionVersion int64
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

	AllApps *bool
	AppIds  []string

	// SessionVersionInc — увеличить session_version на 1 (отозвать все
	// выданные токены). Только для Update; само значение напрямую не задаётся.
	SessionVersionInc bool
}

// ListReq — параметры выборки
type ListReq struct {
	commonModel.ListParams

	Active  *bool
	IsAdmin *bool
	Search  *string
	AppId   *string // только с доступом к этому app (см. session.AccessibleAppIds)
}
