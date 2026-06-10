package model

// Session — данные авторизованного пользователя, извлечённые из JWT-токена.
// Это stateless-сессия: в БД не хранится, целиком кодируется в подписанном токене.
type Session struct {
	Id    int64
	Admin bool
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
