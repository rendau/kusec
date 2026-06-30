package service

import (
	"context"
	"fmt"

	"github.com/pquerna/otp/totp"

	"github.com/rendau/kusec/internal/constant"
	"github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

// generateTotpSecret создаёт новый TOTP-секрет и otpauth-URL для привязки в
// приложении-аутентификаторе.
func generateTotpSecret(username string) (secret, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      constant.ServiceName,
		AccountName: username,
	})
	if err != nil {
		return "", "", fmt.Errorf("totp.Generate: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTotpCode проверяет одноразовый код против секрета (с допуском ±1
// период на рассинхрон часов).
func (s *Service) ValidateTotpCode(secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}

// EnrollTotp генерирует и сохраняет новый секрет для пользователя (ещё не
// включая 2FA — ждём подтверждения через ConfirmTotp). Если 2FA уже включена —
// сначала нужно её отключить/сбросить.
func (s *Service) EnrollTotp(ctx context.Context, usrId int64) (secret, url string, err error) {
	item, _, err := s.Get(ctx, usrId, true)
	if err != nil {
		return "", "", fmt.Errorf("Get: %w", err)
	}
	if item.TotpEnabled {
		return "", "", errs.TotpAlreadyOn
	}

	secret, url, err = generateTotpSecret(item.Username)
	if err != nil {
		return "", "", fmt.Errorf("generateTotpSecret: %w", err)
	}

	if err = s.repoDb.Update(ctx, usrId, &model.Edit{
		TotpSecret:  &secret,
		TotpEnabled: new(false),
	}); err != nil {
		return "", "", fmt.Errorf("repoDb.Update: %w", err)
	}

	return secret, url, nil
}

// ConfirmTotp проверяет первый код против сохранённого секрета и включает 2FA.
// Возвращает обновлённого пользователя (для выдачи токенов).
func (s *Service) ConfirmTotp(ctx context.Context, usrId int64, code string) (*model.Main, error) {
	item, _, err := s.Get(ctx, usrId, true)
	if err != nil {
		return nil, fmt.Errorf("Get: %w", err)
	}
	if item.TotpSecret == "" {
		return nil, errs.TotpNotEnabled
	}
	if !s.ValidateTotpCode(item.TotpSecret, code) {
		return nil, errs.TotpInvalid
	}

	if err = s.repoDb.Update(ctx, usrId, &model.Edit{TotpEnabled: new(true)}); err != nil {
		return nil, fmt.Errorf("repoDb.Update: %w", err)
	}

	item.TotpEnabled = true
	return item, nil
}

// DisableTotp выключает 2FA после проверки текущего кода.
func (s *Service) DisableTotp(ctx context.Context, usrId int64, code string) error {
	item, _, err := s.Get(ctx, usrId, true)
	if err != nil {
		return fmt.Errorf("Get: %w", err)
	}
	if !item.TotpEnabled {
		return errs.TotpNotEnabled
	}
	if !s.ValidateTotpCode(item.TotpSecret, code) {
		return errs.TotpInvalid
	}

	return s.resetTotp(ctx, usrId)
}

// ResetTotp принудительно сбрасывает 2FA пользователя (admin-reset).
func (s *Service) ResetTotp(ctx context.Context, usrId int64) error {
	if _, _, err := s.Get(ctx, usrId, true); err != nil {
		return fmt.Errorf("Get: %w", err)
	}
	return s.resetTotp(ctx, usrId)
}

// ResetTotpByUsername сбрасывает 2FA по username (break-glass CLI). Возвращает
// false, если пользователь не найден.
func (s *Service) ResetTotpByUsername(ctx context.Context, username string) (bool, error) {
	item, found, err := s.repoDb.GetByUsername(ctx, username)
	if err != nil {
		return false, fmt.Errorf("repoDb.GetByUsername: %w", err)
	}
	if !found {
		return false, nil
	}
	if err = s.resetTotp(ctx, item.Id); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) resetTotp(ctx context.Context, usrId int64) error {
	if err := s.repoDb.Update(ctx, usrId, &model.Edit{
		TotpSecret:  new(""),
		TotpEnabled: new(false),
	}); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}
