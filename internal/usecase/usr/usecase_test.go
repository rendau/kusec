package usr

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/kusec/internal/constant"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	sessionService "github.com/rendau/kusec/internal/domain/session/service"
	"github.com/rendau/kusec/internal/domain/usr/model"
	usrService "github.com/rendau/kusec/internal/domain/usr/service"
	"github.com/rendau/kusec/internal/errs"
)

const (
	testJwtSecret = "test-secret"
	testPassword  = "Corr3ct-Horse!"
)

// ── Моки ────────────────────────────────────────────────

// repoMock — хранилище пользователей в памяти для настоящего доменного
// сервиса usr (пароли, TOTP и номер поколения сессий проверяются вместе с
// ним).
type repoMock struct {
	items  map[int64]*model.Main
	lastId int64
}

func (r *repoMock) List(context.Context, *model.ListReq) ([]*model.Main, int64, error) {
	result := make([]*model.Main, 0, len(r.items))
	for _, item := range r.items {
		cp := *item
		result = append(result, &cp)
	}
	return result, int64(len(result)), nil
}

func (r *repoMock) Get(_ context.Context, id int64) (*model.Main, bool, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, false, nil
	}
	cp := *item
	return &cp, true, nil
}

func (r *repoMock) GetByUsername(_ context.Context, username string) (*model.Main, bool, error) {
	for _, item := range r.items {
		if item.Username == username {
			cp := *item
			return &cp, true, nil
		}
	}
	return nil, false, nil
}

func (r *repoMock) HasAny(context.Context) (bool, error) { return len(r.items) > 0, nil }

func (r *repoMock) Create(_ context.Context, obj *model.Edit) (int64, error) {
	r.lastId = max(r.lastId, 100) + 1
	item := &model.Main{Id: r.lastId}
	r.items[item.Id] = item
	applyEdit(item, obj)
	return item.Id, nil
}

func (r *repoMock) Update(_ context.Context, id int64, obj *model.Edit) error {
	applyEdit(r.items[id], obj)
	return nil
}

func (r *repoMock) Delete(_ context.Context, id int64) error {
	delete(r.items, id)
	return nil
}

func (r *repoMock) ApplyValueEncryption(context.Context) (int64, error) { return 0, nil }

func applyEdit(item *model.Main, obj *model.Edit) {
	if obj.Active != nil {
		item.Active = *obj.Active
	}
	if obj.IsAdmin != nil {
		item.IsAdmin = *obj.IsAdmin
	}
	if obj.Name != nil {
		item.Name = *obj.Name
	}
	if obj.Username != nil {
		item.Username = *obj.Username
	}
	if obj.Password != nil {
		item.Password = *obj.Password
	}
	if obj.TotpEnabled != nil {
		item.TotpEnabled = *obj.TotpEnabled
	}
	if obj.TotpSecret != nil {
		item.TotpSecret = *obj.TotpSecret
	}
	if obj.AllApps != nil {
		item.AllApps = *obj.AllApps
	}
	if obj.AppIds != nil {
		item.AppIds = slices.Clone(obj.AppIds)
	}
	if obj.SessionVersionInc {
		item.SessionVersion++
	}
}

type txmStub struct{}

func (txmStub) TxFn(ctx context.Context, f func(context.Context) error) error { return f(ctx) }

type auditRecStub struct{}

func (auditRecStub) RecordUsr(context.Context, *model.Main, *model.Main, *string) error { return nil }

// fixture — usecase поверх настоящих доменного сервиса и сервиса сессий.
type fixture struct {
	uc         *Usecase
	repo       *repoMock
	usrSvc     *usrService.Service
	sessionSvc *sessionService.Service
	// now — часы ограничителя перебора
	now *time.Time
}

func newEnv(t *testing.T) *fixture {
	t.Helper()
	repo := &repoMock{items: map[int64]*model.Main{}}
	sessionSvc := sessionService.New(testJwtSecret)
	usrSvc := usrService.New(repo)
	uc := New(usrSvc, sessionSvc, txmStub{}, auditRecStub{})

	now := time.Now()
	uc.limiter.now = func() time.Time { return now }

	return &fixture{uc: uc, repo: repo, usrSvc: usrSvc, sessionSvc: sessionSvc, now: &now}
}

// addUser создаёт активного пользователя с паролем testPassword.
func (f *fixture) addUser(t *testing.T, username string, edit model.Edit) int64 {
	t.Helper()
	if edit.Active == nil {
		edit.Active = new(true)
	}
	edit.Name = new(username)
	edit.Username = new(username)
	edit.Password = new(testPassword)
	id, err := f.usrSvc.Create(t.Context(), &edit)
	require.NoError(t, err)
	return id
}

// ctxOf — контекст с сессией, как её строит session-интерсептор.
func (f *fixture) ctxOf(t *testing.T, session *sessionModel.Session) context.Context {
	t.Helper()
	return f.sessionSvc.WithContext(t.Context(), session)
}

func (f *fixture) login(t *testing.T, username, password, code string) (*LoginResult, error) {
	t.Helper()
	return f.uc.Login(t.Context(), &LoginReq{
		Username: username,
		Password: password,
		TotpCode: code,
		ClientIp: "203.0.113.7",
	})
}

// enableTotp включает пользователю 2FA и возвращает секрет.
func (f *fixture) enableTotp(t *testing.T, usrId int64) string {
	t.Helper()
	secret, _, err := f.usrSvc.EnrollTotp(t.Context(), usrId)
	require.NoError(t, err)
	f.repo.items[usrId].TotpEnabled = true
	return secret
}

func totpCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	require.NoError(t, err)
	return code
}

// wrongTotpCode — код, который не подходит ни к одному из ближайших шагов
// времени.
func wrongTotpCode(t *testing.T, secret string) string {
	t.Helper()
	valid := map[string]bool{}
	for shift := -3; shift <= 3; shift++ {
		code, err := totp.GenerateCode(secret, time.Now().UTC().Add(time.Duration(shift)*30*time.Second))
		require.NoError(t, err)
		valid[code] = true
	}
	for _, candidate := range []string{"000000", "000001", "000002", "000003", "000004", "000005", "000006", "000007"} {
		if !valid[candidate] {
			return candidate
		}
	}
	t.Fatal("no wrong code candidate")
	return ""
}

// requireErrCode — ошибка ErrFull с заданным кодом; возвращает её поля.
func requireErrCode(t *testing.T, err error, code errs.Err) map[string]string {
	t.Helper()
	require.Error(t, err)
	fullErr, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, "expected errs.ErrFull, got: %v", err)
	require.Equal(t, code, fullErr.Err)
	return fullErr.Fields
}

// ── 2FA ─────────────────────────────────────────────────

// Выключенный, но не удалённый пользователь с живым access-токеном не может
// выпустить себе новые токены через 2FA.
func TestTotp_InactiveUserRejected(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	// сессия построена интерсептором ещё до того, как пользователя выключили
	ctx := f.ctxOf(t, &sessionModel.Session{Id: usrId})

	secret, _, err := f.uc.EnrollTotp(ctx, testPassword)
	require.NoError(t, err)

	f.repo.items[usrId].Active = false // админ выключил пользователя

	_, err = f.uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.ErrorIs(t, err, errs.NotAuthorized)
	assert.False(t, f.repo.items[usrId].TotpEnabled)

	_, _, err = f.uc.EnrollTotp(ctx, testPassword)
	require.ErrorIs(t, err, errs.NotAuthorized)

	require.ErrorIs(t, f.uc.DisableTotp(ctx, totpCode(t, secret)), errs.NotAuthorized)
}

// Подтверждение уже включённой 2FA не выдаёт новых токенов: иначе по одному
// коду из приложения можно продлевать сессию без конца.
func TestConfirmTotp_OnlyOnce(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	ctx := f.ctxOf(t, &sessionModel.Session{Id: usrId})

	secret, _, err := f.uc.EnrollTotp(ctx, testPassword)
	require.NoError(t, err)

	res, err := f.uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
	assert.True(t, f.repo.items[usrId].TotpEnabled)

	_, err = f.uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.ErrorIs(t, err, errs.TotpAlreadyOn)
}

// Привязка 2FA — только с верным текущим паролем. Иначе украденного на
// четверть часа access-токена хватило бы, чтобы привязать к учётке без 2FA
// чужой аутентификатор. Неверные пароли считаются тем же счётчиком, что и
// текущий пароль в профиле.
func TestEnrollTotp_RequiresCurrentPassword(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	ctx := f.ctxOf(t, &sessionModel.Session{Id: usrId})

	// без пароля — сколько угодно раз, попыткой подбора это не считается
	for range attemptMaxFailures * 2 {
		for name, password := range map[string]string{"empty": "", "blank": "   "} {
			_, _, err := f.uc.EnrollTotp(ctx, password)
			require.ErrorIs(t, err, errs.CurrentPasswordInvalid, name)
		}
	}
	assert.Empty(t, f.repo.items[usrId].TotpSecret, "секрет без пароля не привязан")

	// неверный пароль — отказ, и попытки считаются
	for range attemptMaxFailures {
		_, _, err := f.uc.EnrollTotp(ctx, "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.CurrentPasswordInvalid)
	}
	assert.Empty(t, f.repo.items[usrId].TotpSecret)

	// блокировка — даже с верным паролем; счётчик общий с профилем
	_, _, err := f.uc.EnrollTotp(ctx, testPassword)
	fields := requireErrCode(t, err, errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])

	err = f.uc.UpdateProfile(ctx, &UpdateProfileReq{Password: new("N3w-Passw0rd!"), CurrentPassword: new(testPassword)})
	requireErrCode(t, err, errs.TooManyAttempts)
	assert.Empty(t, f.repo.items[usrId].TotpSecret)

	// блокировка истекла — с верным паролем секрет выдан
	*f.now = f.now.Add(time.Minute)
	secret, url, err := f.uc.EnrollTotp(ctx, testPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, url, "otpauth://totp/")
	assert.Equal(t, secret, f.repo.items[usrId].TotpSecret)
	assert.False(t, f.repo.items[usrId].TotpEnabled, "до подтверждения кодом 2FA не включена")
}

// Включение 2FA закрывает все прежние сессии пользователя — в том числе
// чужую, по украденному токену, — и возвращает пару токенов уже нового
// поколения: текущая сессия продолжается на ней.
func TestConfirmTotp_RevokesOldTokensAndIssuesNew(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})

	stolen, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	own, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)

	session, err := f.uc.SessionFromToken(t.Context(), own.Jwt)
	require.NoError(t, err)
	ctx := f.ctxOf(t, session)

	secret, _, err := f.uc.EnrollTotp(ctx, testPassword)
	require.NoError(t, err)

	// привязка без подтверждения сессий не трогает
	assert.EqualValues(t, 0, f.repo.items[usrId].SessionVersion)
	_, err = f.uc.SessionFromToken(t.Context(), stolen.Jwt)
	require.NoError(t, err)

	fresh, err := f.uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.repo.items[usrId].SessionVersion)

	for name, old := range map[string]*LoginResult{"stolen": stolen, "own": own} {
		_, err = f.uc.SessionFromToken(t.Context(), old.Jwt)
		require.ErrorIs(t, err, errs.NotAuthorized, "access: "+name)
		_, _, err = f.uc.RefreshToken(t.Context(), old.RefreshToken)
		require.ErrorIs(t, err, errs.NotAuthorized, "refresh: "+name)
	}

	// возвращённая пара — рабочая, с новым номером поколения
	require.NotEmpty(t, fresh.Jwt)
	require.NotEmpty(t, fresh.RefreshToken)
	session, err = f.uc.SessionFromToken(t.Context(), fresh.Jwt)
	require.NoError(t, err)
	assert.Equal(t, usrId, session.Id)
	_, _, err = f.uc.RefreshToken(t.Context(), fresh.RefreshToken)
	require.NoError(t, err)

	claims := jwtv5.MapClaims{}
	_, _, err = jwtv5.NewParser().ParseUnverified(fresh.Jwt, claims)
	require.NoError(t, err)
	assert.EqualValues(t, 1, claims["ver"])
}

// Отключение 2FA действующим кодом сессий не закрывает: код — подтверждение,
// что это сам пользователь.
func TestDisableTotp_KeepsSessions(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})

	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)

	// 2FA включена мимо usecase: само включение сессии закрыло бы
	secret := f.enableTotp(t, usrId)

	require.NoError(t, f.uc.DisableTotp(f.ctxOf(t, session), totpCode(t, secret)))
	assert.False(t, f.repo.items[usrId].TotpEnabled)
	assert.EqualValues(t, 0, f.repo.items[usrId].SessionVersion)

	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err, "access-токен после отключения 2FA")
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.NoError(t, err, "refresh-токен после отключения 2FA")
}

// Сброс 2FA админом закрывает все сессии целевого пользователя.
func TestResetTotp_RevokesTargetSessions(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	usrId := f.addUser(t, "bob", model.Edit{})
	adminCtx := f.ctxOf(t, &sessionModel.Session{Id: adminId, Admin: true})

	secret := f.enableTotp(t, usrId)
	res, err := f.login(t, "bob", testPassword, totpCode(t, secret))
	require.NoError(t, err)

	require.NoError(t, f.uc.ResetTotp(adminCtx, usrId))

	assert.False(t, f.repo.items[usrId].TotpEnabled)
	assert.Empty(t, f.repo.items[usrId].TotpSecret)
	assert.EqualValues(t, 1, f.repo.items[usrId].SessionVersion)
	assert.EqualValues(t, 0, f.repo.items[adminId].SessionVersion, "сессии админа не тронуты")

	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized)

	// вход теперь без второго фактора
	res, err = f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
}

// Подбор кода 2FA вошедшим пользователем ограничен так же, как вход: пять
// неверных кодов подряд — блокировка, даже верный код не принимается.
func TestTotpCodeChecks_Limited(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	ctx := f.ctxOf(t, &sessionModel.Session{Id: usrId})

	secret := f.enableTotp(t, usrId)
	wrong := wrongTotpCode(t, secret)

	for range attemptMaxFailures {
		require.ErrorIs(t, f.uc.DisableTotp(ctx, wrong), errs.TotpInvalid)
	}

	fields := requireErrCode(t, f.uc.DisableTotp(ctx, totpCode(t, secret)), errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])
	assert.True(t, f.repo.items[usrId].TotpEnabled, "2FA не отключена")

	// блокировка истекла — верный код работает
	*f.now = f.now.Add(time.Minute)
	require.NoError(t, f.uc.DisableTotp(ctx, totpCode(t, secret)))
	assert.False(t, f.repo.items[usrId].TotpEnabled)
}

// ── Вход ────────────────────────────────────────────────

// Пять неверных паролей подряд блокируют вход на минуту: пока блокировка
// действует, не принимается и верный пароль. Успешный вход сбрасывает счётчик.
func TestLogin_LocksAfterFailures(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	_, err := f.login(t, "bob", testPassword, "")
	fields := requireErrCode(t, err, errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])

	// пробелы по краям логина не важны — та же учётка и тот же счётчик
	_, err = f.login(t, "  bob ", testPassword, "")
	requireErrCode(t, err, errs.TooManyAttempts)

	// а регистр важен, как и в базе: BOB — другой логин (такой учётки нет),
	// блокировка bob на него не распространяется
	_, err = f.login(t, "BOB", testPassword, "")
	require.ErrorIs(t, err, errs.NotAuthorized)

	// блокировка истекла: верный пароль проходит и сбрасывает счётчик
	*f.now = f.now.Add(time.Minute)
	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
	assert.NotEmpty(t, res.RefreshToken)

	for range attemptMaxFailures - 1 {
		_, err = f.login(t, "bob", "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}
	_, err = f.login(t, "bob", testPassword, "")
	require.NoError(t, err, "после сброса снова пять попыток")
}

// Счётчик входа ведётся по пользователю, а не по логину без учёта регистра.
// Атака: база различает admin и Admin, обычный пользователь переименовывает
// себя в Admin — и каждым своим успешным входом сбрасывает счётчик неудач
// учётки admin: её пароль подбирается без блокировки.
func TestLogin_OtherCaseLoginDoesNotResetCounter(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "admin", model.Edit{IsAdmin: new(true)})
	bobId := f.addUser(t, "bob", model.Edit{})
	bobCtx := f.ctxOf(t, &sessionModel.Session{Id: bobId})

	// обычный пользователь называет себя Admin — логин свободен: регистр другой
	require.NoError(t, f.uc.UpdateProfile(bobCtx, &UpdateProfileReq{
		Username:        new("Admin"),
		CurrentPassword: new(testPassword),
	}))

	for range attemptMaxFailures - 1 {
		_, err := f.login(t, "admin", "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	// успешный вход под Admin — это вход другого пользователя
	res, err := f.login(t, "Admin", testPassword, "")
	require.NoError(t, err)
	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)
	require.Equal(t, bobId, session.Id)

	// счётчик admin не сброшен: пятая неудача блокирует
	_, err = f.login(t, "admin", "Wrong-Passw0rd!", "")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.login(t, "admin", testPassword, "")
	fields := requireErrCode(t, err, errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])

	// а блокировка admin не мешает входить под Admin
	_, err = f.login(t, "Admin", testPassword, "")
	require.NoError(t, err)
}

// Смена логина счётчик не сбрасывает: он привязан к пользователю.
func TestLogin_RenameKeepsCounter(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures - 1 {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	f.repo.items[usrId].Username = "bobby"

	_, err := f.login(t, "bobby", "Wrong-Passw0rd!", "")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.login(t, "bobby", testPassword, "")
	requireErrCode(t, err, errs.TooManyAttempts)
}

// Поток входов по случайным логинам не вытесняет счётчик существующего
// пользователя: четыре попытки, «смыть» счётчик чужими логинами, ещё четыре —
// так блокировка не наступала бы никогда.
func TestLogin_UnknownLoginsDoNotEvictUserCounter(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.uc.limiter.maxUnknown = 4
	f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures - 1 {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	// логинов втрое больше, чем вмещает карта несуществующих логинов
	for i := range 3 * f.uc.limiter.maxUnknown {
		*f.now = f.now.Add(time.Millisecond)
		_, err := f.login(t, fmt.Sprintf("ghost-%d", i), "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}
	require.Len(t, f.uc.limiter.unknown, f.uc.limiter.maxUnknown)

	_, err := f.login(t, "bob", "Wrong-Passw0rd!", "")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.login(t, "bob", testPassword, "")
	requireErrCode(t, err, errs.TooManyAttempts)
}

// Одновременных проверок пароля на входе — не больше, чем мест: запрос, не
// дождавшийся места, получает service_not_available. Пароль при этом не
// проверялся, поэтому неудачей попытка не считается.
func TestLogin_BusyGate(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.uc.loginGate = newCheckGate(1, 30*time.Millisecond)
	f.addUser(t, "bob", model.Edit{})

	// единственное место занято чужой проверкой
	require.True(t, f.uc.loginGate.acquire(t.Context()))

	for _, username := range []string{"bob", "no-such-user"} {
		for range attemptMaxFailures * 2 {
			_, err := f.login(t, username, testPassword, "")
			fullErr, ok := errors.AsType[errs.ErrFull](err)
			require.True(t, ok, "%s: expected errs.ErrFull, got: %v", username, err)
			require.Equal(t, errs.ServiceNA, fullErr.Err, username)
			assert.NotEmpty(t, fullErr.Desc)
		}
	}
	assert.Empty(t, f.uc.limiter.users, "отказ из-за занятости — не неудача")
	assert.Empty(t, f.uc.limiter.unknown)

	// место освободилось — вход работает
	f.uc.loginGate.release()
	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
}

// После первой блокировки каждая следующая неудача удваивает срок.
func TestLogin_LockEscalates(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!", "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	*f.now = f.now.Add(time.Minute)
	_, err := f.login(t, "bob", "Wrong-Passw0rd!", "")
	require.ErrorIs(t, err, errs.NotAuthorized)

	_, err = f.login(t, "bob", testPassword, "")
	fields := requireErrCode(t, err, errs.TooManyAttempts)
	assert.Equal(t, "120", fields["retry_after"])
}

// Несуществующий логин неотличим от существующего: тот же отказ и та же
// блокировка после пяти попыток (иначе по блокировке видно, есть ли учётка).
func TestLogin_UnknownUserCountedLikeExisting(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{})

	for _, username := range []string{"bob", "no-such-user"} {
		for range attemptMaxFailures {
			_, err := f.login(t, username, "Wrong-Passw0rd!", "")
			require.ErrorIs(t, err, errs.NotAuthorized, username)
		}
		_, err := f.login(t, username, "Wrong-Passw0rd!", "")
		fields := requireErrCode(t, err, errs.TooManyAttempts)
		assert.Equal(t, "60", fields["retry_after"], username)
	}
}

// Выключенная учётка: отказ тот же, что при неверном пароле, и попытка
// засчитывается — по счётчику пароль выключенной учётки не проверить.
func TestLogin_InactiveCountedAsFailure(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{Active: new(false)})

	for range attemptMaxFailures {
		_, err := f.login(t, "bob", testPassword, "")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}
	_, err := f.login(t, "bob", testPassword, "")
	requireErrCode(t, err, errs.TooManyAttempts)
}

// Код 2FA нельзя использовать повторно: второй вход с тем же кодом — отказ.
func TestLogin_TotpCodeNotReusable(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	secret := f.enableTotp(t, usrId)

	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	require.True(t, res.TotpRequired)
	assert.Empty(t, res.Jwt)

	code := totpCode(t, secret)

	res, err = f.login(t, "bob", testPassword, code)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)

	// подсмотренный код второй раз не проходит
	_, err = f.login(t, "bob", testPassword, code)
	require.ErrorIs(t, err, errs.TotpInvalid)
}

// Неверные коды 2FA считаются так же, как неверные пароли; запрос без кода
// («нужен второй фактор») счётчик не сбрасывает — иначе код перебирался бы
// без ограничений.
func TestLogin_TotpFailuresLimited(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	secret := f.enableTotp(t, usrId)
	wrong := wrongTotpCode(t, secret)

	for range attemptMaxFailures - 1 {
		_, err := f.login(t, "bob", testPassword, wrong)
		require.ErrorIs(t, err, errs.TotpInvalid)

		// верный пароль без кода — не успех и не неудача
		res, err := f.login(t, "bob", testPassword, "")
		require.NoError(t, err)
		require.True(t, res.TotpRequired)
	}

	_, err := f.login(t, "bob", testPassword, wrong)
	require.ErrorIs(t, err, errs.TotpInvalid)

	_, err = f.login(t, "bob", testPassword, totpCode(t, secret))
	requireErrCode(t, err, errs.TooManyAttempts)

	// и запрос без кода во время блокировки не проходит
	_, err = f.login(t, "bob", testPassword, "")
	requireErrCode(t, err, errs.TooManyAttempts)
}

// ── Сессия по живой записи ──────────────────────────────

// Сессия строится по записи пользователя на каждый запрос: права меняются
// сразу, выключенный и удалённый пользователь отвергаются до истечения токена.
func TestSessionFromToken_LiveRecord(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{AppIds: []string{"app1"}})

	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)

	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)
	assert.Equal(t, usrId, session.Id)
	assert.False(t, session.Admin)
	assert.False(t, session.AllApps)
	assert.Equal(t, []string{"app1"}, session.AppIds)
	assert.Equal(t, "bob", session.Name)
	assert.Equal(t, constant.SourceUi, session.Source)
	assert.Empty(t, session.ApiKeyId)

	// права в токене не зашиты: смена прав в записи видна тем же токеном
	f.repo.items[usrId].IsAdmin = true
	f.repo.items[usrId].AllApps = true
	f.repo.items[usrId].AppIds = []string{}
	session, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)
	assert.True(t, session.Admin)
	assert.True(t, session.AllApps)
	assert.Empty(t, session.AppIds)

	// refresh-токен как access не принимается
	_, err = f.uc.SessionFromToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized)

	// мусор и токен с чужой подписью
	_, err = f.uc.SessionFromToken(t.Context(), "not-a-token")
	require.ErrorIs(t, err, errs.NotAuthorized)
	foreign, err := sessionService.New("other-secret").CreateToken(usrId, 0)
	require.NoError(t, err)
	_, err = f.uc.SessionFromToken(t.Context(), foreign)
	require.ErrorIs(t, err, errs.NotAuthorized)

	// выключенный пользователь
	f.repo.items[usrId].Active = false
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)
	f.repo.items[usrId].Active = true

	// удалённый пользователь
	delete(f.repo.items, usrId)
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)
}

// В access-токене нет прав — только id пользователя и номер поколения сессий.
func TestAccessToken_CarriesNoPermissions(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "root", model.Edit{IsAdmin: new(true), AppIds: []string{"app1"}})

	res, err := f.login(t, "root", testPassword, "")
	require.NoError(t, err)

	for name, token := range map[string]string{"access": res.Jwt, "refresh": res.RefreshToken} {
		claims := jwtv5.MapClaims{}
		_, _, err = jwtv5.NewParser().ParseUnverified(token, claims)
		require.NoError(t, err, name)

		assert.NotContains(t, claims, "is_admin", name)
		assert.NotContains(t, claims, "app_ids", name)
		assert.EqualValues(t, usrId, claims["id"], name)
		assert.EqualValues(t, 0, claims["ver"], name)
	}
}

// Выход гасит все токены пользователя: и access, и refresh.
func TestLogout_RevokesTokens(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{})

	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)

	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)

	require.NoError(t, f.uc.Logout(f.ctxOf(t, session)))

	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized, "access-токен после выхода")
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized, "refresh-токен после выхода")

	// новый вход выдаёт рабочие токены
	res, err = f.login(t, "bob", testPassword, "")
	require.NoError(t, err)
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)
	access, refresh, err := f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)

	// без входа выходить некому
	require.ErrorIs(t, f.uc.Logout(t.Context()), errs.NotAuthorized)
}

// Смена пароля — самим пользователем или админом — отзывает выданные токены.
func TestPasswordChange_RevokesTokens(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	usrId := f.addUser(t, "bob", model.Edit{})
	adminCtx := f.ctxOf(t, &sessionModel.Session{Id: adminId, Admin: true})

	const newPassword = "N3w-Passw0rd!"

	// админ меняет пароль пользователю
	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)

	// правка без пароля токены не трогает
	require.NoError(t, f.uc.Update(adminCtx, usrId, &model.Edit{Name: new("Bob")}))
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)

	require.NoError(t, f.uc.Update(adminCtx, usrId, &model.Edit{Password: new(newPassword)}))
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized)

	// пользователь меняет пароль сам
	res, err = f.login(t, "bob", newPassword, "")
	require.NoError(t, err)
	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)

	require.NoError(t, f.uc.UpdateProfile(f.ctxOf(t, session), &UpdateProfileReq{
		Password:        new(testPassword),
		CurrentPassword: new(newPassword),
	}))
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized)
}

// Выключение пользователя отзывает его токены насовсем: повторное включение
// старые токены не оживляет.
func TestDeactivation_RevokesTokens(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	usrId := f.addUser(t, "bob", model.Edit{})
	adminCtx := f.ctxOf(t, &sessionModel.Session{Id: adminId, Admin: true})

	res, err := f.login(t, "bob", testPassword, "")
	require.NoError(t, err)

	require.NoError(t, f.uc.Update(adminCtx, usrId, &model.Edit{Active: new(false)}))
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)

	require.NoError(t, f.uc.Update(adminCtx, usrId, &model.Edit{Active: new(true)}))
	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized)
}

// Токены, выданные до появления claim ver, считаются поколением 0: работают,
// пока пользователь не вышел и не сменил пароль.
func TestLegacyTokenWithoutVer(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})

	now := time.Now().UTC()
	legacy, err := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, jwtv5.MapClaims{
		"typ":      "access",
		"id":       usrId,
		"is_admin": true, // права из старого токена не учитываются
		"app_ids":  []string{},
		"iat":      now.Unix(),
		"exp":      now.Add(time.Minute).Unix(),
	}).SignedString([]byte(testJwtSecret))
	require.NoError(t, err)

	session, err := f.uc.SessionFromToken(t.Context(), legacy)
	require.NoError(t, err)
	assert.Equal(t, usrId, session.Id)
	assert.False(t, session.Admin, "is_admin из старого токена не даёт прав")

	require.NoError(t, f.usrSvc.RevokeSessions(t.Context(), usrId))
	_, err = f.uc.SessionFromToken(t.Context(), legacy)
	require.ErrorIs(t, err, errs.NotAuthorized)
}

// ── Профиль ─────────────────────────────────────────────

// Пароль и логин меняются только с верным текущим паролем; имя — без него.
func TestUpdateProfile_RequiresCurrentPassword(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	ctx := f.ctxOf(t, &sessionModel.Session{Id: usrId})
	oldHash := f.repo.items[usrId].Password

	const newPassword = "N3w-Passw0rd!"

	// смена пароля: без текущего, с пустым, с неверным
	for name, current := range map[string]*string{
		"nil":   nil,
		"empty": new(""),
		"blank": new("   "),
		"wrong": new("Wrong-Passw0rd!"),
	} {
		err := f.uc.UpdateProfile(ctx, &UpdateProfileReq{Password: new(newPassword), CurrentPassword: current})
		require.ErrorIs(t, err, errs.CurrentPasswordInvalid, name)
	}
	assert.Equal(t, oldHash, f.repo.items[usrId].Password, "пароль не изменён")

	// смена логина — то же самое
	err := f.uc.UpdateProfile(ctx, &UpdateProfileReq{Username: new("bobby")})
	require.ErrorIs(t, err, errs.CurrentPasswordInvalid)
	err = f.uc.UpdateProfile(ctx, &UpdateProfileReq{Username: new("bobby"), CurrentPassword: new("Wrong-Passw0rd!")})
	require.ErrorIs(t, err, errs.CurrentPasswordInvalid)
	assert.Equal(t, "bob", f.repo.items[usrId].Username)

	// имя меняется без текущего пароля; форма профиля шлёт и прежний логин —
	// это не смена логина
	require.NoError(t, f.uc.UpdateProfile(ctx, &UpdateProfileReq{Name: new("Bob"), Username: new(" bob ")}))
	assert.Equal(t, "Bob", f.repo.items[usrId].Name)

	// с верным текущим паролем меняются и логин, и пароль
	require.NoError(t, f.uc.UpdateProfile(ctx, &UpdateProfileReq{
		Username:        new("bobby"),
		Password:        new(newPassword),
		CurrentPassword: new(testPassword),
	}))
	assert.Equal(t, "bobby", f.repo.items[usrId].Username)
	assert.NotEqual(t, oldHash, f.repo.items[usrId].Password)

	_, err = f.login(t, "bobby", newPassword, "")
	require.NoError(t, err)
}

// Подбор текущего пароля ограничен: пять неверных подряд — блокировка, не
// принимается и верный. Пустой пароль попыткой не считается.
func TestUpdateProfile_CurrentPasswordAttemptsLimited(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	ctx := f.ctxOf(t, &sessionModel.Session{Id: usrId})

	const newPassword = "N3w-Passw0rd!"
	change := func(current *string) error {
		return f.uc.UpdateProfile(ctx, &UpdateProfileReq{Password: new(newPassword), CurrentPassword: current})
	}

	// пустые — сколько угодно, счётчик не растёт
	for range attemptMaxFailures * 2 {
		require.ErrorIs(t, change(nil), errs.CurrentPasswordInvalid)
	}

	for range attemptMaxFailures {
		require.ErrorIs(t, change(new("Wrong-Passw0rd!")), errs.CurrentPasswordInvalid)
	}

	fields := requireErrCode(t, change(new(testPassword)), errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])

	// привязка 2FA спрашивает тот же пароль под тем же счётчиком
	_, _, err := f.uc.EnrollTotp(ctx, testPassword)
	requireErrCode(t, err, errs.TooManyAttempts)

	// успешная проверка кода 2FA счётчик текущего пароля не сбрасывает
	// (секрет привязан мимо usecase: привязка сейчас заблокирована)
	secret, _, err := f.usrSvc.EnrollTotp(t.Context(), usrId)
	require.NoError(t, err)
	_, err = f.uc.ConfirmTotp(ctx, totpCode(t, secret))
	require.NoError(t, err)
	requireErrCode(t, change(new(testPassword)), errs.TooManyAttempts)

	*f.now = f.now.Add(time.Minute)
	require.NoError(t, change(new(testPassword)))
}

// ── Права ───────────────────────────────────────────────

// Сессия по API-ключу или OAuth-подключению учётками не управляет; читать
// свой профиль может.
func TestApiKeySession_CannotManageAccounts(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	usrId := f.addUser(t, "bob", model.Edit{})

	// ключ админа: права владельца, но это не вход в админку
	keyCtx := f.ctxOf(t, &sessionModel.Session{
		Id: adminId, Admin: true, Name: "root", Source: constant.SourceApi,
		ApiKeyId: "key-1", ApiKeyName: "ci",
	})

	secret := f.enableTotp(t, adminId)

	calls := map[string]func() error{
		"UpdateProfile": func() error {
			return f.uc.UpdateProfile(keyCtx, &UpdateProfileReq{
				Password:        new("N3w-Passw0rd!"),
				CurrentPassword: new(testPassword),
			})
		},
		"UpdateProfile(name)": func() error {
			return f.uc.UpdateProfile(keyCtx, &UpdateProfileReq{Name: new("Root")})
		},
		"EnrollTotp": func() error {
			_, _, err := f.uc.EnrollTotp(keyCtx, testPassword)
			return err
		},
		"ConfirmTotp": func() error {
			_, err := f.uc.ConfirmTotp(keyCtx, totpCode(t, secret))
			return err
		},
		"DisableTotp": func() error { return f.uc.DisableTotp(keyCtx, totpCode(t, secret)) },
		"Logout":      func() error { return f.uc.Logout(keyCtx) },
		"Create": func() error {
			_, err := f.uc.Create(keyCtx, &model.Edit{
				Name: new("Eve"), Username: new("eve"), Password: new(testPassword), IsAdmin: new(true),
			})
			return err
		},
		"Update":    func() error { return f.uc.Update(keyCtx, usrId, &model.Edit{IsAdmin: new(true)}) },
		"Delete":    func() error { return f.uc.Delete(keyCtx, usrId) },
		"ResetTotp": func() error { return f.uc.ResetTotp(keyCtx, adminId) },
	}
	for name, call := range calls {
		require.ErrorIs(t, call(), errs.NoPermission, name)
	}

	// ничего не изменилось
	assert.True(t, f.repo.items[adminId].TotpEnabled)
	assert.EqualValues(t, 0, f.repo.items[adminId].SessionVersion)
	assert.False(t, f.repo.items[usrId].IsAdmin)
	assert.Len(t, f.repo.items, 2)

	// чтение оставлено: свой профиль и (админу) список пользователей
	profile, err := f.uc.GetProfile(keyCtx)
	require.NoError(t, err)
	assert.Equal(t, "root", profile.Username)
	assert.Empty(t, profile.Password)

	_, _, err = f.uc.List(keyCtx, &model.ListReq{})
	require.NoError(t, err)
}

// Список пользователей и чужие учётки — только админу; свои данные не-админ
// получает через GetProfile.
func TestUsrListGet_AdminOnly(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	usrId := f.addUser(t, "bob", model.Edit{})

	usrCtx := f.ctxOf(t, &sessionModel.Session{Id: usrId})
	adminCtx := f.ctxOf(t, &sessionModel.Session{Id: adminId, Admin: true})

	_, _, err := f.uc.List(usrCtx, &model.ListReq{})
	require.ErrorIs(t, err, errs.NoPermission)
	_, err = f.uc.Get(usrCtx, adminId)
	require.ErrorIs(t, err, errs.NoPermission)
	_, err = f.uc.Get(usrCtx, usrId)
	require.ErrorIs(t, err, errs.NoPermission, "даже себя — только через GetProfile")

	profile, err := f.uc.GetProfile(usrCtx)
	require.NoError(t, err)
	assert.Equal(t, "bob", profile.Username)

	// без входа — not_authorized
	_, _, err = f.uc.List(t.Context(), &model.ListReq{})
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.uc.Get(t.Context(), usrId)
	require.ErrorIs(t, err, errs.NotAuthorized)

	items, total, err := f.uc.List(adminCtx, &model.ListReq{})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	for _, item := range items {
		assert.Empty(t, item.Password)
	}
	item, err := f.uc.Get(adminCtx, usrId)
	require.NoError(t, err)
	assert.Equal(t, "bob", item.Username)
	assert.Empty(t, item.Password)
}

// Свои пароль, логин и 2FA админ меняет только в профиле (там нужен текущий
// пароль или действующий код). Через управление пользователями — отказ: это
// был бы обход тех проверок для украденной сессии админа. Остальные свои
// поля менять можно, свой логин без изменения — не отказ.
func TestAdminCannotChangeOwnCredentialsViaUpdate(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	otherAdminId := f.addUser(t, "root2", model.Edit{IsAdmin: new(true)})
	adminCtx := f.ctxOf(t, &sessionModel.Session{Id: adminId, Admin: true})
	otherAdminCtx := f.ctxOf(t, &sessionModel.Session{Id: otherAdminId, Admin: true})

	secret := f.enableTotp(t, adminId)
	oldHash := f.repo.items[adminId].Password

	requireSelfDenied := func(err error, what string) {
		t.Helper()
		fullErr, ok := errors.AsType[errs.ErrFull](err)
		require.True(t, ok, "%s: expected errs.ErrFull, got: %v", what, err)
		require.Equal(t, errs.NoPermission, fullErr.Err, what)
		assert.Equal(t, "change your own password, username and 2FA in the profile", fullErr.Desc, what)
	}

	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{Password: new("N3w-Passw0rd!")}), "свой пароль")
	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{Username: new("superroot")}), "свой логин")
	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{Username: new("ROOT")}), "свой логин в другом регистре")
	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{
		Name: new("Root"), Username: new("root"), Password: new("N3w-Passw0rd!"),
	}), "пароль вместе с прежним логином")
	requireSelfDenied(f.uc.ResetTotp(adminCtx, adminId), "сброс своей 2FA")

	// ничего не изменилось, сессии не отозваны
	self := f.repo.items[adminId]
	assert.Equal(t, oldHash, self.Password)
	assert.Equal(t, "root", self.Username)
	assert.Equal(t, "root", self.Name)
	assert.True(t, self.TotpEnabled)
	assert.Equal(t, secret, self.TotpSecret)
	assert.EqualValues(t, 0, self.SessionVersion)

	// остальные свои поля — можно; логин без изменения (форма шлёт его
	// всегда, пробелы по краям не в счёт) — не отказ
	require.NoError(t, f.uc.Update(adminCtx, adminId, &model.Edit{Name: new("Root")}))
	require.NoError(t, f.uc.Update(adminCtx, adminId, &model.Edit{Name: new("Root User"), Username: new(" root ")}))
	assert.Equal(t, "Root User", f.repo.items[adminId].Name)
	assert.Equal(t, "root", f.repo.items[adminId].Username)
	assert.Equal(t, oldHash, f.repo.items[adminId].Password)

	// чужую учётку админ правит как раньше — в том числе другой админ эту
	require.NoError(t, f.uc.Update(adminCtx, otherAdminId, &model.Edit{Password: new("N3w-Passw0rd!"), Username: new("root-two")}))
	assert.Equal(t, "root-two", f.repo.items[otherAdminId].Username)

	require.NoError(t, f.uc.ResetTotp(otherAdminCtx, adminId))
	assert.False(t, f.repo.items[adminId].TotpEnabled)
	require.NoError(t, f.uc.Update(otherAdminCtx, adminId, &model.Edit{Password: new("N3w-Passw0rd!")}))
	assert.NotEqual(t, oldHash, f.repo.items[adminId].Password)
}

// Без входа (токена нет либо он истёк) админские действия отвечают
// not_authorized, а не no_permission: по этому коду клиент обновляет токен.
func TestAdminActions_AnonymousNotAuthorized(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})
	usrCtx := f.ctxOf(t, &sessionModel.Session{Id: usrId})

	calls := map[string]func(ctx context.Context) error{
		"Update":    func(ctx context.Context) error { return f.uc.Update(ctx, usrId, &model.Edit{Name: new("Bob")}) },
		"Delete":    func(ctx context.Context) error { return f.uc.Delete(ctx, usrId) },
		"ResetTotp": func(ctx context.Context) error { return f.uc.ResetTotp(ctx, usrId) },
		"Create": func(ctx context.Context) error {
			_, err := f.uc.Create(ctx, &model.Edit{Name: new("Eve"), Username: new("eve"), Password: new(testPassword)})
			return err
		},
	}
	for name, call := range calls {
		// сессии нет вовсе и пустая сессия (токен не принят)
		require.ErrorIs(t, call(t.Context()), errs.NotAuthorized, name)
		require.ErrorIs(t, call(f.ctxOf(t, &sessionModel.Session{})), errs.NotAuthorized, name)
		// вошедший не-админ — по-прежнему no_permission
		require.ErrorIs(t, call(usrCtx), errs.NoPermission, name)
	}
	assert.Len(t, f.repo.items, 1)
	assert.Equal(t, "bob", f.repo.items[usrId].Name)
}
