package app

import (
	"fmt"
)

// ResetUserTotp — break-glass: принудительно отключает 2FA пользователю по
// username. Нужен, когда админ потерял аутентификатор, а сбросить 2FA некому
// (например, админ в системе один). Требует доступа к серверу/бинарнику.
// Вызывается после Init() (пул БД и сервисы уже подняты).
func (a *App) ResetUserTotp(username string) error {
	found, err := a.usrSvc.ResetTotpByUsername(a.ctx, username)
	if err != nil {
		return fmt.Errorf("usrSvc.ResetTotpByUsername: %w", err)
	}
	if !found {
		return fmt.Errorf("user not found: %s", username)
	}
	return nil
}
