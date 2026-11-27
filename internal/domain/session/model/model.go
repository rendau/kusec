package model

import (
	"github.com/rendau/kusec/internal/constant"
)

// Session — данные авторизованного пользователя. Строится на каждый запрос
// по живой записи пользователя: по id из JWT-токена входа либо по API-ключу /
// OAuth-подключению. Права в самом токене не хранятся, в БД сессия не лежит.
type Session struct {
	Id    int64
	Admin bool
	// AllApps — не-админ с доступом ко всем приложениям. Иначе доступны
	// только AppIds; пустой список = не доступно ничего.
	AllApps bool
	AppIds  []string

	// Name — имя пользователя (для аудита). Если пусто — аудит резолвит имя
	// сам по Id.
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

// SecretValuesHidden — вызывающему значения item-ов секретов не показываются:
// read_only-скоуп (usecase обнуляет Value) или MCP-канал (handler отдаёт
// только маску). Таким сессиям нельзя давать и поиск по значению — иначе
// фильтр «значение содержит …» становится оракулом для посимвольного перебора.
func (s *Session) SecretValuesHidden() bool {
	return s != nil && (s.IsReadOnly() || s.Source == constant.SourceMcp)
}

// ConfigValuesHidden — значения config_item-ов не показываются (только
// read_only-скоуп: MCP значения конфигмапов видит целиком).
func (s *Session) ConfigValuesHidden() bool {
	return s != nil && s.IsReadOnly()
}

// AccessibleAppIds — к каким приложениям у сессии есть доступ: (nil, true) —
// ко всем (админ или AllApps); (список, false) — только к перечисленным.
//
// ВАЖНО: (пустой список, false) значит «ни к одному», а не «ко всем».
// Репозитории пустой срез фильтром не считают, поэтому вызывающий обязан
// сам вернуть пустой результат или отказ и не передавать пустой список
// дальше как фильтр.
func (s *Session) AccessibleAppIds() ([]string, bool) {
	if s == nil || !s.IsAuthorized() {
		return nil, false
	}
	if s.Admin || s.AllApps {
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
