package model

import (
	"github.com/rendau/kusec/internal/constant"
)

// Session — данные авторизованного пользователя, извлечённые из JWT-токена
// или построенные по API-ключу. Это stateless-сессия: в БД не хранится.
type Session struct {
	Id     int64
	Admin  bool
	AppIds []string

	// Name — имя пользователя (для аудита); заполняется при аутентификации
	// по API-ключу. Для JWT-сессий пусто — аудит резолвит имя сам.
	Name string

	// Scope доступа: constant.ApiKeyScope*. Пусто = full (JWT-сессии
	// и ключи без ограничений).
	Scope string

	// Source — канал, по которому пришёл запрос: constant.Source*.
	Source string

	// ApiKeyId / ApiKeyName — ключ, по которому построена сессия
	// (пусто для JWT-сессий). Денормализуются в аудит.
	ApiKeyId   string
	ApiKeyName string
}

func New(id int64) *Session {
	return &Session{Id: id}
}

func (s *Session) IsAuthorized() bool {
	return s != nil && s.Id > 0
}

func (s *Session) IsAdmin() bool {
	return s != nil && s.IsAuthorized() && s.Admin
}

// IsReadOnly — сессия ограничена скоупом read_only (маскирование значений,
// белый список методов).
func (s *Session) IsReadOnly() bool {
	return s != nil && s.Scope == constant.ApiKeyScopeReadOnly
}

func (s *Session) AccessibleAppIds() ([]string, bool) {
	if s == nil {
		return nil, false
	}
	if s.Admin {
		return nil, true
	}
	if len(s.AppIds) == 0 {
		return nil, true
	}
	return s.AppIds, false
}

func (s *Session) HasAppAccess(appId string) bool {
	appIds, all := s.AccessibleAppIds()
	if all {
		return true
	}
	for _, id := range appIds {
		if id == appId {
			return true
		}
	}
	return false
}
