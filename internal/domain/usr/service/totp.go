package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/rendau/kusec/internal/constant"
	"github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

const (
	// totpPeriod — шаг времени TOTP в секундах (как у totp.Generate и
	// приложений-аутентификаторов).
	totpPeriod = 30
	// totpGuardSweepSize — с какого размера карты защиты от повторов из неё
	// выметаются записи, которые уже ничего не блокируют.
	totpGuardSweepSize = 1024
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

// matchTotpStep определяет шаг времени, которому соответствует код: текущий
// либо соседний (допуск ±1 шаг на рассинхрон часов). Проверка каждого шага
// отдельно (Skew: 0) нужна, чтобы узнать сам шаг — totp.Validate его не
// возвращает.
func matchTotpStep(secret, code string, now time.Time) (int64, bool) {
	opts := totp.ValidateOpts{
		Period:    totpPeriod,
		Skew:      0,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	}
	for _, shift := range []time.Duration{0, -totpPeriod * time.Second, totpPeriod * time.Second} {
		t := now.Add(shift)
		if ok, err := totp.ValidateCustom(code, secret, t, opts); err == nil && ok {
			return t.Unix() / totpPeriod, true
		}
	}
	return 0, false
}

// totpReplayGuard помнит последний принятый шаг времени каждого пользователя
// (в памяти процесса): код с тем же или более ранним шагом второй раз не
// принимается. Подсмотренный или перехваченный код нельзя использовать, пока
// он ещё действителен (до 90 секунд с допуском на рассинхрон).
type totpReplayGuard struct {
	mu   sync.Mutex
	last map[int64]int64 // usrId → последний принятый шаг
}

func newTotpReplayGuard() *totpReplayGuard {
	return &totpReplayGuard{last: map[int64]int64{}}
}

// accept отмечает шаг принятым; false — шаг уже использован. curStep —
// текущий шаг времени (для чистки устаревших записей).
func (g *totpReplayGuard) accept(usrId, step, curStep int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if last, ok := g.last[usrId]; ok && step <= last {
		return false
	}
	g.last[usrId] = step

	// запись нужна, только пока её шаг ещё может прийти снова (curStep-1 и
	// новее); более старые выметаем, чтобы карта не росла
	if len(g.last) > totpGuardSweepSize {
		for id, s := range g.last {
			if s < curStep-1 {
				delete(g.last, id)
			}
		}
	}
	return true
}

// forget сбрасывает запись пользователя: секрет заменён или удалён, старые
// шаги к новому секрету не относятся.
func (g *totpReplayGuard) forget(usrId int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.last, usrId)
}

// ValidateTotpCode проверяет одноразовый код пользователя против секрета (с
// допуском ±1 период на рассинхрон часов). Каждый код принимается один раз:
// повтор кода с уже использованным шагом времени — отказ.
func (s *Service) ValidateTotpCode(usrId int64, secret, code string) bool {
	if secret == "" || code == "" {
		return false
	}

	now := s.now().UTC()
	step, ok := matchTotpStep(secret, code, now)
	if !ok {
		return false
	}
	return s.totpGuard.accept(usrId, step, now.Unix()/totpPeriod)
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
	s.totpGuard.forget(usrId)

	return secret, url, nil
}

// ConfirmTotp проверяет первый код против сохранённого секрета и включает 2FA.
// Включение 2FA отзывает все выданные пользователю токены (session_version +
// 1): сессии, открытые до появления второго фактора, — в том числе чужая, по
// украденному токену, — закрываются. Возвращает обновлённого пользователя:
// по нему вызывающий выдаёт новую пару токенов уже с новым номером.
func (s *Service) ConfirmTotp(ctx context.Context, usrId int64, code string) (*model.Main, error) {
	item, _, err := s.Get(ctx, usrId, true)
	if err != nil {
		return nil, fmt.Errorf("Get: %w", err)
	}
	if item.TotpSecret == "" {
		return nil, errs.TotpNotEnabled
	}
	// повторное подтверждение уже включённой 2FA не нужно и выдавало бы
	// новые токены по одному коду
	if item.TotpEnabled {
		return nil, errs.TotpAlreadyOn
	}
	if !s.ValidateTotpCode(usrId, item.TotpSecret, code) {
		return nil, errs.TotpInvalid
	}

	if err = s.repoDb.Update(ctx, usrId, &model.Edit{
		TotpEnabled:       new(true),
		SessionVersionInc: true,
	}); err != nil {
		return nil, fmt.Errorf("repoDb.Update: %w", err)
	}

	// номер поколения сессий вырос в базе — запись перечитывается, а не
	// правится на месте
	item, _, err = s.Get(ctx, usrId, true)
	if err != nil {
		return nil, fmt.Errorf("Get: %w", err)
	}
	return item, nil
}

// DisableTotp выключает 2FA после проверки текущего кода. Сессии не
// отзываются: действующий код — подтверждение, что это сам пользователь.
func (s *Service) DisableTotp(ctx context.Context, usrId int64, code string) error {
	item, _, err := s.Get(ctx, usrId, true)
	if err != nil {
		return fmt.Errorf("Get: %w", err)
	}
	if !item.TotpEnabled {
		return errs.TotpNotEnabled
	}
	if !s.ValidateTotpCode(usrId, item.TotpSecret, code) {
		return errs.TotpInvalid
	}

	return s.resetTotp(ctx, usrId, false)
}

// ResetTotp принудительно сбрасывает 2FA пользователя (admin-reset) и
// отзывает все выданные ему токены: сброс второго фактора без кода делают,
// когда устройство потеряно или учётка под подозрением, — старые сессии при
// этом жить не должны.
func (s *Service) ResetTotp(ctx context.Context, usrId int64) error {
	if _, _, err := s.Get(ctx, usrId, true); err != nil {
		return fmt.Errorf("Get: %w", err)
	}
	return s.resetTotp(ctx, usrId, true)
}

// ResetTotpByUsername сбрасывает 2FA по username (break-glass CLI) — так же,
// как admin-reset, с отзывом выданных токенов. Возвращает false, если
// пользователь не найден.
func (s *Service) ResetTotpByUsername(ctx context.Context, username string) (bool, error) {
	item, found, err := s.repoDb.GetByUsername(ctx, username)
	if err != nil {
		return false, fmt.Errorf("repoDb.GetByUsername: %w", err)
	}
	if !found {
		return false, nil
	}
	if err = s.resetTotp(ctx, item.Id, true); err != nil {
		return false, err
	}
	return true, nil
}

// resetTotp убирает секрет и выключает 2FA; revokeSessions — заодно отозвать
// все выданные пользователю токены (session_version + 1) тем же запросом.
func (s *Service) resetTotp(ctx context.Context, usrId int64, revokeSessions bool) error {
	if err := s.repoDb.Update(ctx, usrId, &model.Edit{
		TotpSecret:        new(""),
		TotpEnabled:       new(false),
		SessionVersionInc: revokeSessions,
	}); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	s.totpGuard.forget(usrId)
	return nil
}
