package usr

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/samber/lo"

	"github.com/rendau/kusec/internal/constant"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

const (
	// passwordMinLen — минимальная длина пароля (в рунах).
	passwordMinLen = 8
	// passwordMaxLen — максимальная длина пароля в байтах: bcrypt молча обрезает
	// всё, что длиннее 72 байт, поэтому ограничиваем явно.
	passwordMaxLen = 72

	// logUsernameMaxLen — предел длины логина в журнале входов (в рунах):
	// логин приходит от клиента и может быть любой длины.
	logUsernameMaxLen = 64
)

// Причины отказа во входе (поле reason журнала «login failed»).
const (
	loginReasonBadCredentials = "bad_credentials"
	loginReasonLocked         = "locked"
	loginReasonInactive       = "inactive"
	loginReasonBusy           = "busy"
)

// selfManagedDesc — отказ администратору, который пытается сменить
// собственные пароль или логин через управление пользователями.
const selfManagedDesc = "change your own password and username in the profile"

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
	txm        TransactionManagerI
	auditRec   AuditRecorderI

	// limiter — ограничитель перебора пароля (в памяти процесса).
	limiter *attemptLimiter
	// loginGate — предел одновременных проверок пароля на входе.
	loginGate *checkGate
}

func New(svc ServiceI, sessionSvc SessionServiceI, txm TransactionManagerI, auditRec AuditRecorderI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
		txm:        txm,
		auditRec:   auditRec,
		limiter:    newAttemptLimiter(),
		loginGate:  newLoginCheckGate(),
	}
}

// auditUsrMutation выполняет мутацию пользователя в транзакции с записью
// аудита: старое/новое состояние читаются вокруг мутации, пустой дифф
// не пишется.
func (u *Usecase) auditUsrMutation(ctx context.Context, id int64, mutate func(ctx context.Context) error) error {
	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		old, _, err := u.svc.Get(ctx, id, false)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}

		if err = mutate(ctx); err != nil {
			return err
		}

		cur, _, err := u.svc.Get(ctx, id, false)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordUsr(ctx, old, cur, nil); err != nil {
			return fmt.Errorf("auditRec.RecordUsr: %w", err)
		}
		return nil
	})
}

// issueTokenPair выдаёт access- и refresh-токен с текущим номером поколения
// сессий пользователя (claim ver): после смены пароля или выхода номер в
// записи растёт, и выданные раньше токены перестают приниматься.
func (u *Usecase) issueTokenPair(item *model.Main) (string, string, error) {
	access, err := u.sessionSvc.CreateToken(item.Id, item.SessionVersion)
	if err != nil {
		return "", "", fmt.Errorf("sessionSvc.CreateToken: %w", err)
	}

	refresh, err := u.sessionSvc.CreateRefreshToken(item.Id, item.Password, item.SessionVersion)
	if err != nil {
		return "", "", fmt.Errorf("sessionSvc.CreateRefreshToken: %w", err)
	}

	return access, refresh, nil
}

func (u *Usecase) issueLoginResult(item *model.Main) (*LoginResult, error) {
	access, refresh, err := u.issueTokenPair(item)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Jwt: access, RefreshToken: refresh}, nil
}

// beginAttempt резервирует попытку в ограничителе перебора либо возвращает
// отказ too_many_attempts.
func (u *Usecase) beginAttempt(key attemptKey) (*attempt, error) {
	a, retryAfter, ok := u.limiter.begin(key)
	if !ok {
		return nil, tooManyAttemptsErr(retryAfter)
	}
	return a, nil
}

// Login проверяет логин и пароль.
//
// Защита от перебора: неудачи (неверный пароль) считаются по
// пользователю — по его id, а для логина, которого нет, по самому логину;
// после 5 подряд вход блокируется (см. attemptLimiter). Пока действует
// блокировка, отказ выдаётся без проверки пароля — даже верного.
// Несуществующий логин неотличим от существующего ни по ответу, ни по
// блокировке, ни по времени (bcrypt выполняется в обоих случаях).
// Одновременных проверок пароля — не больше, чем мест в loginGate: лишние
// запросы ждут и, не дождавшись, получают service_not_available.
// Каждая неудача пишется в журнал с причиной, успех — тоже.
func (u *Usecase) Login(ctx context.Context, req *LoginReq) (*LoginResult, error) {
	if req == nil {
		return nil, errs.InvalidRequest
	}
	username := strings.TrimSpace(req.Username)

	logFailed := func(reason string) {
		slog.Warn("login failed",
			"username", lo.Substring(username, 0, logUsernameMaxLen),
			"client_ip", req.ClientIp,
			"reason", reason,
		)
	}

	// учётка ищется до ограничителя: от того, есть ли она, зависит ключ счётчика
	item, found, err := u.svc.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("svc.GetByUsername: %w", err)
	}

	key := unknownLoginAttemptKey(username)
	if found {
		key = loginAttemptKey(item.Id)
	}

	attempt, err := u.beginAttempt(key)
	if err != nil {
		logFailed(loginReasonLocked)
		return nil, err
	}
	defer attempt.release()

	passwordOk, err := u.checkLoginPassword(ctx, item, req.Password)
	if err != nil {
		if fullErr, ok := errors.AsType[errs.ErrFull](err); ok && errors.Is(fullErr.Err, errs.ServiceNA) {
			logFailed(loginReasonBusy)
		}
		return nil, err
	}
	if !passwordOk {
		attempt.fail()
		logFailed(loginReasonBadCredentials)
		return nil, errs.NotAuthorized
	}
	if !item.Active {
		// засчитываем как неудачу: иначе по тому, растёт ли счётчик, можно
		// было бы проверить пароль выключенной учётки
		attempt.fail()
		logFailed(loginReasonInactive)
		return nil, errs.NotAuthorized
	}

	res, err := u.issueLoginResult(item)
	if err != nil {
		return nil, err
	}

	attempt.succeed()
	slog.Info("login ok",
		"username", lo.Substring(username, 0, logUsernameMaxLen),
		"client_ip", req.ClientIp,
		"usr_id", item.Id,
	)

	return res, nil
}

// checkLoginPassword сверяет пароль на входе; item == nil — логина нет, и
// выполняется «холостое» сравнение той же цены. Проверка идёт под loginGate:
// не дождавшись места, вход получает service_not_available (пароль при этом
// не проверялся — неудачей попытка не считается).
func (u *Usecase) checkLoginPassword(ctx context.Context, item *model.Main, password string) (bool, error) {
	if !u.loginGate.acquire(ctx) {
		return false, errs.ErrFull{
			Err:  errs.ServiceNA,
			Desc: "too many sign-in requests right now, try again in a few seconds",
		}
	}
	defer u.loginGate.release()

	if item == nil {
		u.svc.BurnPasswordCheck(password)
		return false, nil
	}

	ok, err := u.svc.PasswordMatches(item.Password, password)
	if err != nil {
		return false, fmt.Errorf("svc.PasswordMatches: %w", err)
	}
	return ok, nil
}

// SessionFromToken строит сессию по access-токену входа. Используется
// session-интерсептором на каждый запрос: токен несёт только id пользователя
// и номер поколения сессий, а права (админ, приложения) берутся из живой
// записи. Выключенный или удалённый пользователь и токен с устаревшим
// номером (после смены пароля или выхода) отвергаются сразу, а не по
// истечении токена; смена прав действует со следующего запроса.
func (u *Usecase) SessionFromToken(ctx context.Context, token string) (*sessionModel.Session, error) {
	usrId, ver, err := u.sessionSvc.ParseAccessToken(token)
	if err != nil {
		return nil, errs.NotAuthorized
	}

	item, found, err := u.svc.Get(ctx, usrId, false)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if !found || !item.Active || item.SessionVersion != ver {
		return nil, errs.NotAuthorized
	}

	return &sessionModel.Session{
		Id:      item.Id,
		Admin:   item.IsAdmin,
		AllApps: item.AllApps,
		AppIds:  item.AppIds,
		Name:    item.Name,
		Source:  constant.SourceUi,
	}, nil
}

// requireHuman — действие доступно только из входа в админку. Сессия по
// API-ключу или OAuth-подключению учётками не управляет: утёкший ключ не
// должен давать смену пароля или выпуск новых ключей.
func requireHuman(session *sessionModel.Session) error {
	if !session.IsAuthorized() {
		return errs.NotAuthorized
	}
	if session.ApiKeyId != "" {
		return errs.NoPermission
	}
	return nil
}

// requireAdminHuman — действие администратора из входа в админку. Без входа
// (нет токена либо он истёк) — not_authorized, а не no_permission: по этому
// коду клиент обновляет токен и повторяет запрос.
func (u *Usecase) requireAdminHuman(ctx context.Context) error {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAuthorized() {
		return errs.NotAuthorized
	}
	if !session.IsAdmin() || session.ApiKeyId != "" {
		return errs.NoPermission
	}
	return nil
}

// requireActiveSelf — текущий пользователь вошёл через админку (не по
// ключу), существует и не выключен. Session-интерсептор уже сверяет
// пользователя по БД на каждый запрос; здесь запись перечитывается ещё раз,
// потому что методы профиля работают с её актуальным состоянием (хэш
// пароля).
func (u *Usecase) requireActiveSelf(ctx context.Context) (*model.Main, error) {
	session := u.sessionSvc.FromContext(ctx)
	if err := requireHuman(session); err != nil {
		return nil, err
	}
	item, found, err := u.svc.Get(ctx, session.Id, false)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if !found || !item.Active {
		return nil, errs.NotAuthorized
	}
	return item, nil
}

// Logout гасит все выданные текущему пользователю токены — access и refresh,
// на всех устройствах: номер поколения сессий растёт, токены с прежним
// номером больше не принимаются.
func (u *Usecase) Logout(ctx context.Context) error {
	item, err := u.requireActiveSelf(ctx)
	if err != nil {
		return err
	}
	if err = u.svc.RevokeSessions(ctx, item.Id); err != nil {
		return fmt.Errorf("svc.RevokeSessions: %w", err)
	}
	slog.Info("logout", "usr_id", item.Id)
	return nil
}

// ConfirmPassword подтверждает текущим паролем действие, выдающее долгий
// доступ в обход сессии: выпуск API-ключа, согласие на OAuth-подключение MCP.
// Ключ и подключение переживают выход и смену пароля, поэтому чужой с живой
// сессией (украденный токен, открытый экран) не должен выпускать их сам.
// Счётчик неудач общий с текущим паролем в профиле.
func (u *Usecase) ConfirmPassword(ctx context.Context, currentPassword string) error {
	item, err := u.requireActiveSelf(ctx)
	if err != nil {
		return err
	}
	return u.checkCurrentPassword(item, &currentPassword)
}

// RefreshToken обменивает валидный refresh-токен на новую пару токенов
// (refresh ротируется). Пользователь перечитывается из БД: выключенный
// пользователь токенов не получает, а смена пароля и выход отзывают ранее
// выданные refresh-токены (несовпадение отпечатка пароля или номера
// поколения сессий).
func (u *Usecase) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return "", "", errs.NotAuthorized
	}

	usrId, err := u.sessionSvc.RefreshTokenUserId(refreshToken)
	if err != nil {
		return "", "", errs.NotAuthorized
	}

	item, found, err := u.svc.Get(ctx, usrId, false)
	if err != nil {
		return "", "", fmt.Errorf("svc.Get: %w", err)
	}
	if !found || !item.Active {
		return "", "", errs.NotAuthorized
	}

	_, ver, err := u.sessionSvc.ParseRefreshToken(refreshToken, item.Password)
	if err != nil || ver != item.SessionVersion {
		return "", "", errs.NotAuthorized
	}

	return u.issueTokenPair(item)
}

// BootstrapStatus сообщает, можно ли создать первого администратора (нет ни одного пользователя).
func (u *Usecase) BootstrapStatus(ctx context.Context) (bool, error) {
	hasAny, err := u.svc.HasAny(ctx)
	if err != nil {
		return false, fmt.Errorf("svc.HasAny: %w", err)
	}
	return !hasAny, nil
}

func (u *Usecase) GetProfile(ctx context.Context) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	session := u.sessionSvc.FromContext(ctx)

	item, _, err := u.svc.Get(ctx, session.Id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if !item.Active {
		return nil, errs.NotAuthorized
	}

	item.Password = ""

	return item, nil
}

// UpdateProfile меняет имя, логин и пароль текущего пользователя. Смена
// пароля или логина — только с верным текущим паролем (current_password):
// чужой с живой сессией (открытый экран, украденный токен) не перехватит
// учётку. Смена пароля отзывает все токены пользователя, включая текущий.
func (u *Usecase) UpdateProfile(ctx context.Context, req *UpdateProfileReq) error {
	item, err := u.requireActiveSelf(ctx)
	if err != nil {
		return err
	}
	if req == nil || (req.Name == nil && req.Username == nil && req.Password == nil) {
		return errs.InvalidRequest
	}

	edit := &model.Edit{
		Name:     req.Name,
		Username: req.Username,
		Password: req.Password,
	}
	if err = u.validateEdit(edit, false); err != nil {
		return err
	}

	// форма профиля шлёт логин всегда — подтверждение нужно, только если он
	// действительно меняется
	usernameChanges := edit.Username != nil && *edit.Username != item.Username
	if edit.Password != nil || usernameChanges {
		if err = u.checkCurrentPassword(item, req.CurrentPassword); err != nil {
			return err
		}
	}

	return u.auditUsrMutation(ctx, item.Id, func(ctx context.Context) error {
		if err = u.svc.Update(ctx, item.Id, edit); err != nil {
			return fmt.Errorf("svc.Update: %w", err)
		}
		return nil
	})
}

// checkCurrentPassword сверяет текущий пароль пользователя под ограничителем
// перебора (смена пароля и логина в профиле, выпуск ключа, согласие OAuth —
// счётчик общий).
// Пустой пароль — отказ без попытки (это не подбор).
func (u *Usecase) checkCurrentPassword(item *model.Main, currentPassword *string) error {
	if currentPassword == nil || strings.TrimSpace(*currentPassword) == "" {
		return errs.CurrentPasswordInvalid
	}

	attempt, err := u.beginAttempt(passwordAttemptKey(item.Id))
	if err != nil {
		slog.Warn("current password check failed", "usr_id", item.Id, "reason", loginReasonLocked)
		return err
	}
	defer attempt.release()

	ok, err := u.svc.PasswordMatches(item.Password, *currentPassword)
	if err != nil {
		return fmt.Errorf("svc.PasswordMatches: %w", err)
	}
	if !ok {
		attempt.fail()
		slog.Warn("current password check failed", "usr_id", item.Id, "reason", loginReasonBadCredentials)
		return errs.CurrentPasswordInvalid
	}

	attempt.succeed()
	return nil
}

// List — список пользователей, только администратору: логины и состав
// учёток остальным не раскрываются (свои данные — через GetProfile).
func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, 0, errs.NoPermission
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}

	for i := range items {
		items[i].Password = ""
	}

	return items, tCount, nil
}

// Get — пользователь по id, только администратору (свои данные — через
// GetProfile).
func (u *Usecase) Get(ctx context.Context, id int64) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return nil, errs.NoPermission
	}
	if id == 0 {
		return nil, errs.IdRequired
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	item.Password = ""

	return item, nil
}

func (u *Usecase) Create(ctx context.Context, obj *model.Edit) (int64, error) {
	if obj == nil {
		obj = &model.Edit{}
	}

	session := u.sessionSvc.FromContext(ctx)
	if session.IsAuthorized() {
		if err := u.requireAdminHuman(ctx); err != nil {
			return 0, err
		}
	} else {
		// неавторизованный — bootstrap первого администратора, только если пользователей нет
		hasAny, err := u.svc.HasAny(ctx)
		if err != nil {
			return 0, fmt.Errorf("svc.HasAny: %w", err)
		}
		if hasAny {
			return 0, errs.NotAuthorized
		}
		obj.IsAdmin = new(true)
		obj.Active = new(true)
	}

	if err := u.validateEdit(obj, true); err != nil {
		return 0, err
	}

	var newId int64
	err := u.txm.TxFn(ctx, func(ctx context.Context) error {
		var err error
		newId, err = u.svc.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("svc.Create: %w", err)
		}

		created, _, err := u.svc.Get(ctx, newId, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if err = u.auditRec.RecordUsr(ctx, nil, created, nil); err != nil {
			return fmt.Errorf("auditRec.RecordUsr: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return newId, nil
}

// Update меняет пользователя (только админ). Собственные пароль и логин
// админ здесь не меняет — только в профиле, где нужен текущий пароль: иначе
// украденной сессии админа хватило бы, чтобы перехватить его учётку.
// Остальные свои поля (имя и т.п.) менять можно; свой логин, переданный без
// изменения (форма шлёт его всегда), — не отказ.
func (u *Usecase) Update(ctx context.Context, id int64, obj *model.Edit) error {
	if err := u.requireAdminHuman(ctx); err != nil {
		return err
	}
	if id == 0 {
		return errs.IdRequired
	}

	if err := u.validateEdit(obj, false); err != nil {
		return err
	}

	if id == u.sessionSvc.FromContext(ctx).Id {
		if err := u.requireNoSelfCredentialChange(ctx, id, obj); err != nil {
			return err
		}
	}

	// app_ids заменяется целиком на каждый Update; пустой список — ни одного
	// приложения (доступ ко всем даёт только all_apps)
	if obj.AppIds == nil {
		obj.AppIds = []string{}
	}

	return u.auditUsrMutation(ctx, id, func(ctx context.Context) error {
		if err := u.svc.Update(ctx, id, obj); err != nil {
			return fmt.Errorf("svc.Update: %w", err)
		}
		return nil
	})
}

// requireNoSelfCredentialChange — в правке собственной учётки админа нет
// смены пароля и логина (obj уже прошёл validateEdit — логин без пробелов по
// краям).
func (u *Usecase) requireNoSelfCredentialChange(ctx context.Context, id int64, obj *model.Edit) error {
	denied := errs.ErrFull{Err: errs.NoPermission, Desc: selfManagedDesc}

	if obj.Password != nil {
		return denied
	}
	if obj.Username != nil {
		self, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}
		if *obj.Username != self.Username {
			return denied
		}
	}
	return nil
}

func (u *Usecase) Delete(ctx context.Context, id int64) error {
	if err := u.requireAdminHuman(ctx); err != nil {
		return err
	}
	if id == 0 {
		return errs.IdRequired
	}

	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		old, _, err := u.svc.Get(ctx, id, true)
		if err != nil {
			return fmt.Errorf("svc.Get: %w", err)
		}

		if err = u.svc.Delete(ctx, id); err != nil {
			return fmt.Errorf("svc.Delete: %w", err)
		}
		if err = u.auditRec.RecordUsr(ctx, old, nil, nil); err != nil {
			return fmt.Errorf("auditRec.RecordUsr: %w", err)
		}
		return nil
	})
}

func (u *Usecase) validateEdit(obj *model.Edit, forCreate bool) error {
	if obj == nil {
		return errs.InvalidRequest
	}

	if forCreate && obj.Name == nil {
		return errs.NameRequired
	}
	if obj.Name != nil {
		*obj.Name = strings.TrimSpace(*obj.Name)
		if *obj.Name == "" {
			return errs.NameRequired
		}
	}

	if forCreate && obj.Username == nil {
		return errs.UsernameRequired
	}
	if obj.Username != nil {
		*obj.Username = strings.TrimSpace(*obj.Username)
		if *obj.Username == "" {
			return errs.UsernameRequired
		}
	}

	if forCreate && obj.Password == nil {
		return errs.PasswordRequired
	}
	if obj.Password != nil {
		*obj.Password = strings.TrimSpace(*obj.Password)
		if *obj.Password == "" {
			return errs.PasswordRequired
		}
		if err := validatePassword(*obj.Password); err != nil {
			return err
		}
	}

	return nil
}

// validatePassword проверяет требования к сложности пароля: длина,
// наличие буквы в верхнем регистре и спецсимвола.
func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < passwordMinLen {
		return errs.PasswordTooShort
	}
	if len(password) > passwordMaxLen {
		return errs.PasswordTooLong
	}

	var hasUpper, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errs.PasswordRequiresUpper
	}
	if !hasSpecial {
		return errs.PasswordRequiresSpecial
	}

	return nil
}
