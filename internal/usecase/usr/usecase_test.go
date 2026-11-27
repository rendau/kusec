package usr

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
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
// сервиса usr (пароли и номер поколения сессий проверяются вместе с ним).
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

func (f *fixture) login(t *testing.T, username, password string) (*LoginResult, error) {
	t.Helper()
	return f.uc.Login(t.Context(), &LoginReq{
		Username: username,
		Password: password,
		ClientIp: "203.0.113.7",
	})
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

// ── Вход ────────────────────────────────────────────────

// Пять неверных паролей подряд блокируют вход на минуту: пока блокировка
// действует, не принимается и верный пароль. Успешный вход сбрасывает счётчик.
func TestLogin_LocksAfterFailures(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	_, err := f.login(t, "bob", testPassword)
	fields := requireErrCode(t, err, errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])

	// пробелы по краям логина не важны — та же учётка и тот же счётчик
	_, err = f.login(t, "  bob ", testPassword)
	requireErrCode(t, err, errs.TooManyAttempts)

	// а регистр важен, как и в базе: BOB — другой логин (такой учётки нет),
	// блокировка bob на него не распространяется
	_, err = f.login(t, "BOB", testPassword)
	require.ErrorIs(t, err, errs.NotAuthorized)

	// блокировка истекла: верный пароль проходит и сбрасывает счётчик
	*f.now = f.now.Add(time.Minute)
	res, err := f.login(t, "bob", testPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
	assert.NotEmpty(t, res.RefreshToken)

	for range attemptMaxFailures - 1 {
		_, err = f.login(t, "bob", "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}
	_, err = f.login(t, "bob", testPassword)
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
		_, err := f.login(t, "admin", "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	// успешный вход под Admin — это вход другого пользователя
	res, err := f.login(t, "Admin", testPassword)
	require.NoError(t, err)
	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)
	require.Equal(t, bobId, session.Id)

	// счётчик admin не сброшен: пятая неудача блокирует
	_, err = f.login(t, "admin", "Wrong-Passw0rd!")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.login(t, "admin", testPassword)
	fields := requireErrCode(t, err, errs.TooManyAttempts)
	assert.Equal(t, "60", fields["retry_after"])

	// а блокировка admin не мешает входить под Admin
	_, err = f.login(t, "Admin", testPassword)
	require.NoError(t, err)
}

// Смена логина счётчик не сбрасывает: он привязан к пользователю.
func TestLogin_RenameKeepsCounter(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures - 1 {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	f.repo.items[usrId].Username = "bobby"

	_, err := f.login(t, "bobby", "Wrong-Passw0rd!")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.login(t, "bobby", testPassword)
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
		_, err := f.login(t, "bob", "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	// логинов втрое больше, чем вмещает карта несуществующих логинов
	for i := range 3 * f.uc.limiter.maxUnknown {
		*f.now = f.now.Add(time.Millisecond)
		_, err := f.login(t, fmt.Sprintf("ghost-%d", i), "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}
	require.Len(t, f.uc.limiter.unknown, f.uc.limiter.maxUnknown)

	_, err := f.login(t, "bob", "Wrong-Passw0rd!")
	require.ErrorIs(t, err, errs.NotAuthorized)
	_, err = f.login(t, "bob", testPassword)
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
			_, err := f.login(t, username, testPassword)
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
	res, err := f.login(t, "bob", testPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Jwt)
}

// После первой блокировки каждая следующая неудача удваивает срок.
func TestLogin_LockEscalates(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	f.addUser(t, "bob", model.Edit{})

	for range attemptMaxFailures {
		_, err := f.login(t, "bob", "Wrong-Passw0rd!")
		require.ErrorIs(t, err, errs.NotAuthorized)
	}

	*f.now = f.now.Add(time.Minute)
	_, err := f.login(t, "bob", "Wrong-Passw0rd!")
	require.ErrorIs(t, err, errs.NotAuthorized)

	_, err = f.login(t, "bob", testPassword)
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
			_, err := f.login(t, username, "Wrong-Passw0rd!")
			require.ErrorIs(t, err, errs.NotAuthorized, username)
		}
		_, err := f.login(t, username, "Wrong-Passw0rd!")
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
		_, err := f.login(t, "bob", testPassword)
		require.ErrorIs(t, err, errs.NotAuthorized)
	}
	_, err := f.login(t, "bob", testPassword)
	requireErrCode(t, err, errs.TooManyAttempts)
}

// ── Сессия по живой записи ──────────────────────────────

// Сессия строится по записи пользователя на каждый запрос: права меняются
// сразу, выключенный и удалённый пользователь отвергаются до истечения токена.
func TestSessionFromToken_LiveRecord(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{AppIds: []string{"app1"}})

	res, err := f.login(t, "bob", testPassword)
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

	res, err := f.login(t, "root", testPassword)
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

	res, err := f.login(t, "bob", testPassword)
	require.NoError(t, err)

	session, err := f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.NoError(t, err)

	require.NoError(t, f.uc.Logout(f.ctxOf(t, session)))

	_, err = f.uc.SessionFromToken(t.Context(), res.Jwt)
	require.ErrorIs(t, err, errs.NotAuthorized, "access-токен после выхода")
	_, _, err = f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.ErrorIs(t, err, errs.NotAuthorized, "refresh-токен после выхода")

	// новый вход выдаёт рабочие токены
	res, err = f.login(t, "bob", testPassword)
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

// Продление не сдвигает время входа: новый refresh-токен несёт время входа
// из старого и истекает не позже входа + общий срок сессии (7 дней).
func TestRefreshToken_KeepsSessionStart(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	usrId := f.addUser(t, "bob", model.Edit{})

	refreshClaims := func(token string) jwtv5.MapClaims {
		t.Helper()
		claims := jwtv5.MapClaims{}
		_, _, err := jwtv5.NewParser().ParseUnverified(token, claims)
		require.NoError(t, err)
		return claims
	}

	// при входе время входа — сейчас, и продление его сохраняет
	res, err := f.login(t, "bob", testPassword)
	require.NoError(t, err)
	loginAst := refreshClaims(res.RefreshToken)["ast"]
	require.NotNil(t, loginAst)
	_, refresh, err := f.uc.RefreshToken(t.Context(), res.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, loginAst, refreshClaims(refresh)["ast"])

	// вход 6 дней назад: продление отдаёт токен до входа + 7 дней
	item := f.repo.items[usrId]
	start := time.Now().Add(-6 * 24 * time.Hour).Truncate(time.Second)
	old, err := f.sessionSvc.CreateRefreshToken(usrId, item.Password, item.SessionVersion, start)
	require.NoError(t, err)
	_, refresh, err = f.uc.RefreshToken(t.Context(), old)
	require.NoError(t, err)
	claims := refreshClaims(refresh)
	assert.EqualValues(t, start.Unix(), claims["ast"])
	assert.EqualValues(t, start.Add(7*24*time.Hour).Unix(), claims["exp"])
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
	res, err := f.login(t, "bob", testPassword)
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
	res, err = f.login(t, "bob", newPassword)
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

	res, err := f.login(t, "bob", testPassword)
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

	_, err = f.login(t, "bobby", newPassword)
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

	// подтверждение паролем (выпуск ключа, согласие OAuth) — тот же счётчик
	requireErrCode(t, f.uc.ConfirmPassword(ctx, testPassword), errs.TooManyAttempts)

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
		"ConfirmPassword": func() error { return f.uc.ConfirmPassword(keyCtx, testPassword) },
		"Logout":          func() error { return f.uc.Logout(keyCtx) },
		"Create": func() error {
			_, err := f.uc.Create(keyCtx, &model.Edit{
				Name: new("Eve"), Username: new("eve"), Password: new(testPassword), IsAdmin: new(true),
			})
			return err
		},
		"Update": func() error { return f.uc.Update(keyCtx, usrId, &model.Edit{IsAdmin: new(true)}) },
		"Delete": func() error { return f.uc.Delete(keyCtx, usrId) },
	}
	for name, call := range calls {
		require.ErrorIs(t, call(), errs.NoPermission, name)
	}

	// ничего не изменилось
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

// Свои пароль и логин админ меняет только в профиле (там нужен текущий
// пароль). Через управление пользователями — отказ: это
// был бы обход тех проверок для украденной сессии админа. Остальные свои
// поля менять можно, свой логин без изменения — не отказ.
func TestAdminCannotChangeOwnCredentialsViaUpdate(t *testing.T) {
	t.Parallel()

	f := newEnv(t)
	adminId := f.addUser(t, "root", model.Edit{IsAdmin: new(true)})
	otherAdminId := f.addUser(t, "root2", model.Edit{IsAdmin: new(true)})
	adminCtx := f.ctxOf(t, &sessionModel.Session{Id: adminId, Admin: true})
	otherAdminCtx := f.ctxOf(t, &sessionModel.Session{Id: otherAdminId, Admin: true})

	oldHash := f.repo.items[adminId].Password

	requireSelfDenied := func(err error, what string) {
		t.Helper()
		fullErr, ok := errors.AsType[errs.ErrFull](err)
		require.True(t, ok, "%s: expected errs.ErrFull, got: %v", what, err)
		require.Equal(t, errs.NoPermission, fullErr.Err, what)
		assert.Equal(t, "change your own password and username in the profile", fullErr.Desc, what)
	}

	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{Password: new("N3w-Passw0rd!")}), "свой пароль")
	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{Username: new("superroot")}), "свой логин")
	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{Username: new("ROOT")}), "свой логин в другом регистре")
	requireSelfDenied(f.uc.Update(adminCtx, adminId, &model.Edit{
		Name: new("Root"), Username: new("root"), Password: new("N3w-Passw0rd!"),
	}), "пароль вместе с прежним логином")

	// ничего не изменилось, сессии не отозваны
	self := f.repo.items[adminId]
	assert.Equal(t, oldHash, self.Password)
	assert.Equal(t, "root", self.Username)
	assert.Equal(t, "root", self.Name)
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
		"Update": func(ctx context.Context) error { return f.uc.Update(ctx, usrId, &model.Edit{Name: new("Bob")}) },
		"Delete": func(ctx context.Context) error { return f.uc.Delete(ctx, usrId) },
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
