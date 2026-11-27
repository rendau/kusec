package service

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTotpTestService — сервис с остановленными часами: проверки кода не
// зависят от того, на какую секунду шага пришёлся запуск теста.
func newTotpTestService(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	// середина шага времени
	now := time.Unix(1_800_000_000-1_800_000_000%totpPeriod+totpPeriod/2, 0).UTC()
	svc := New(nil)
	svc.now = func() time.Time { return now }
	return svc, &now
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, at)
	require.NoError(t, err)
	return code
}

// wrongCode — код, который заведомо не подходит ни к текущему, ни к соседним
// шагам времени.
func wrongCode(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	valid := map[string]bool{
		codeAt(t, secret, now.Add(-totpPeriod*time.Second)): true,
		codeAt(t, secret, now):                              true,
		codeAt(t, secret, now.Add(totpPeriod*time.Second)):  true,
	}
	for _, candidate := range []string{"000000", "000001", "000002", "000003"} {
		if !valid[candidate] {
			return candidate
		}
	}
	t.Fatal("no wrong code candidate")
	return ""
}

func TestGenerateTotpSecret(t *testing.T) {
	secret, url, err := generateTotpSecret("alice")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, url, "otpauth://totp/")
	assert.Contains(t, url, "kusec")
}

func TestValidateTotpCode(t *testing.T) {
	svc, now := newTotpTestService(t)

	secret, _, err := generateTotpSecret("alice")
	require.NoError(t, err)

	code := codeAt(t, secret, *now)

	assert.False(t, svc.ValidateTotpCode(1, secret, wrongCode(t, secret, *now)), "wrong code must fail")
	assert.False(t, svc.ValidateTotpCode(1, secret, ""), "empty code must fail")
	assert.False(t, svc.ValidateTotpCode(1, "", code), "empty secret must fail")
	assert.True(t, svc.ValidateTotpCode(1, secret, code), "valid code must pass")
}

// Шаг времени, которому соответствует код: текущий и соседние (±1) — да,
// дальше — нет.
func TestMatchTotpStep(t *testing.T) {
	_, now := newTotpTestService(t)

	secret, _, err := generateTotpSecret("alice")
	require.NoError(t, err)

	curStep := now.Unix() / totpPeriod

	for shift, wantOk := range map[int64]bool{-2: false, -1: true, 0: true, 1: true, 2: false} {
		at := now.Add(time.Duration(shift*totpPeriod) * time.Second)
		step, ok := matchTotpStep(secret, codeAt(t, secret, at), *now)
		assert.Equal(t, wantOk, ok, "shift=%d", shift)
		if wantOk {
			assert.Equal(t, curStep+shift, step, "shift=%d", shift)
		}
	}
}

// Код 2FA принимается один раз: повтор того же кода и код более раннего шага
// отвергаются, код следующего шага проходит.
func TestValidateTotpCode_NoReplay(t *testing.T) {
	svc, now := newTotpTestService(t)

	secret, _, err := generateTotpSecret("alice")
	require.NoError(t, err)

	prev := codeAt(t, secret, now.Add(-totpPeriod*time.Second))
	cur := codeAt(t, secret, *now)
	next := codeAt(t, secret, now.Add(totpPeriod*time.Second))

	require.True(t, svc.ValidateTotpCode(1, secret, cur), "первое использование кода")
	assert.False(t, svc.ValidateTotpCode(1, secret, cur), "повтор того же кода")
	assert.False(t, svc.ValidateTotpCode(1, secret, prev), "код более раннего шага после принятого")

	// у другого пользователя свой счётчик шагов
	assert.True(t, svc.ValidateTotpCode(2, secret, cur), "другой пользователь")

	assert.True(t, svc.ValidateTotpCode(1, secret, next), "код следующего шага")
	assert.False(t, svc.ValidateTotpCode(1, secret, next), "повтор кода следующего шага")

	// время ушло вперёд — новый код снова принимается, старые нет
	*now = now.Add(3 * totpPeriod * time.Second)
	assert.False(t, svc.ValidateTotpCode(1, secret, next), "устаревший код")
	assert.True(t, svc.ValidateTotpCode(1, secret, codeAt(t, secret, *now)), "свежий код")
}

// Неверный код принятый шаг не сдвигает: после опечатки верный код проходит.
func TestValidateTotpCode_WrongCodeDoesNotBurnStep(t *testing.T) {
	svc, now := newTotpTestService(t)

	secret, _, err := generateTotpSecret("alice")
	require.NoError(t, err)

	assert.False(t, svc.ValidateTotpCode(1, secret, wrongCode(t, secret, *now)))
	assert.True(t, svc.ValidateTotpCode(1, secret, codeAt(t, secret, *now)))
}

// Замена или сброс секрета забывает принятый шаг: код нового секрета с тем
// же шагом времени — не повтор.
func TestTotpReplayGuard_ForgetOnSecretChange(t *testing.T) {
	guard := newTotpReplayGuard()

	require.True(t, guard.accept(1, 100, 100))
	require.False(t, guard.accept(1, 100, 100))

	guard.forget(1)
	assert.True(t, guard.accept(1, 100, 100))
}

// Карта принятых шагов не растёт без конца: устаревшие записи выметаются.
func TestTotpReplayGuard_SweepsStale(t *testing.T) {
	guard := newTotpReplayGuard()

	const oldStep = 100
	for id := int64(1); id <= totpGuardSweepSize; id++ {
		require.True(t, guard.accept(id, oldStep, oldStep))
	}
	require.Len(t, guard.last, totpGuardSweepSize)

	// много шагов спустя: новая запись запускает чистку
	const newStep = oldStep + 10
	require.True(t, guard.accept(totpGuardSweepSize+1, newStep, newStep))
	assert.Len(t, guard.last, 1)

	// свежие записи (текущий и предыдущий шаг) чистку переживают
	guard = newTotpReplayGuard()
	for id := int64(1); id <= totpGuardSweepSize; id++ {
		require.True(t, guard.accept(id, newStep-1, newStep))
	}
	require.True(t, guard.accept(totpGuardSweepSize+1, newStep, newStep))
	assert.Len(t, guard.last, totpGuardSweepSize+1)
}
