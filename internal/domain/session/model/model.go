package model

// Session — данные авторизованного пользователя, извлечённые из JWT-токена.
// Это stateless-сессия: в БД не хранится, целиком кодируется в подписанном токене.
type Session struct {
	Id     int64
	Admin  bool
	AppIds []string
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
