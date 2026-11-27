package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
	usrDb "github.com/rendau/kusec/internal/domain/usr/repo/db"
	usrService "github.com/rendau/kusec/internal/domain/usr/service"
	"github.com/rendau/kusec/internal/infra/secretbox"
)

const totpTestPassword = "Totp-Test-Password-1!"

// newTotpUser создаёт пользователя и привязывает ему ключ 2FA; возвращает id
// и открытый ключ, как его получил бы пользователь при привязке.
func newTotpUser(t *testing.T, svc *usrService.Service, username string) (int64, string) {
	t.Helper()
	ctx := context.Background()

	usrId, err := svc.Create(ctx, &usrModel.Edit{
		Active:   new(true),
		Name:     new(username),
		Username: new(username),
		Password: new(totpTestPassword),
	})
	require.NoError(t, err)

	secret, _, err := svc.EnrollTotp(ctx, usrId)
	require.NoError(t, err)
	require.NotEmpty(t, secret)

	return usrId, secret
}

// rawTotpSecret — ключ 2FA как он лежит в таблице, мимо репозитория.
func (s *stack) rawTotpSecret(t *testing.T, usrId int64) string {
	t.Helper()
	var value string
	err := s.pool.QueryRow(context.Background(), `select totp_secret from usr where id = $1`, usrId).Scan(&value)
	require.NoError(t, err)
	return value
}

// Ключ 2FA, записанный через сервис, лежит в таблице шифртекстом; любое
// чтение (по id, списком, по логину) возвращает исходный ключ. Сброс 2FA
// оставляет пустую строку — пустое не шифруется.
func TestTotpSecretIsEncryptedAtRest(t *testing.T) {
	dsn := isolatedDSN(t)
	ctx := context.Background()

	s := newStackWith(t, dsn, encBox(t, 1))
	svc := usrService.New(usrDb.New(s.pool, s.box))

	usrId, secret := newTotpUser(t, svc, "totp-enc")

	raw := s.rawTotpSecret(t, usrId)
	assert.True(t, secretbox.IsSealed(raw), "totp_secret must be stored encrypted")
	assert.NotContains(t, raw, secret)

	got, _, err := svc.Get(ctx, usrId, true)
	require.NoError(t, err)
	assert.Equal(t, secret, got.TotpSecret)

	list, _, err := svc.List(ctx, &usrModel.ListReq{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, secret, list[0].TotpSecret)

	byLogin, found, err := svc.GetByUsername(ctx, "totp-enc")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, secret, byLogin.TotpSecret)

	require.NoError(t, svc.ResetTotp(ctx, usrId))
	assert.Empty(t, s.rawTotpSecret(t, usrId))
}

// Перевод при старте: открытый ключ 2FA из базы без шифрования шифруется
// один раз, повторный запуск ничего не меняет; без ключа и с чужим ключом
// сервис на такой базе не стартует.
func TestApplyValueEncryptionSealsLegacyTotpSecrets(t *testing.T) {
	dsn := isolatedDSN(t)
	ctx := context.Background()

	// база «до включения шифрования»
	plain := newStackWith(t, dsn, noBox(t))
	plainSvc := usrService.New(usrDb.New(plain.pool, plain.box))

	usrId, secret := newTotpUser(t, plainSvc, "totp-legacy")
	// пользователь без 2FA: пустой ключ переводить не нужно
	noTotpId, err := plainSvc.Create(ctx, &usrModel.Edit{
		Active: new(true), Name: new("no-totp"), Username: new("no-totp"), Password: new(totpTestPassword),
	})
	require.NoError(t, err)

	require.Equal(t, secret, plain.rawTotpSecret(t, usrId))

	sealedCount, err := plainSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.Zero(t, sealedCount)

	// включили ключ
	sealed := newStackWith(t, dsn, encBox(t, 1))
	sealedSvc := usrService.New(usrDb.New(sealed.pool, sealed.box))

	sealedCount, err = sealedSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, sealedCount)

	raw := sealed.rawTotpSecret(t, usrId)
	assert.True(t, secretbox.IsSealed(raw))
	assert.False(t, strings.Contains(raw, secret))
	assert.Empty(t, sealed.rawTotpSecret(t, noTotpId))

	got, _, err := sealedSvc.Get(ctx, usrId, true)
	require.NoError(t, err)
	assert.Equal(t, secret, got.TotpSecret)

	// повторный старт ничего не меняет
	sealedCount, err = sealedSvc.ApplyValueEncryption(ctx)
	require.NoError(t, err)
	assert.Zero(t, sealedCount)
	assert.Equal(t, raw, sealed.rawTotpSecret(t, usrId))

	// старт без ключа — ошибка, чтение — тоже, а не шифртекст вместо ключа
	_, err = plainSvc.ApplyValueEncryption(ctx)
	require.ErrorIs(t, err, secretbox.ErrKeyRequired)
	_, _, err = plainSvc.Get(ctx, usrId, true)
	require.ErrorIs(t, err, secretbox.ErrKeyRequired)

	// старт с чужим ключом — ошибка до любых изменений
	wrong := newStackWith(t, dsn, encBox(t, 2))
	wrongSvc := usrService.New(usrDb.New(wrong.pool, wrong.box))
	_, err = wrongSvc.ApplyValueEncryption(ctx)
	require.ErrorIs(t, err, secretbox.ErrDecrypt)
	assert.Equal(t, raw, wrong.rawTotpSecret(t, usrId))
}
