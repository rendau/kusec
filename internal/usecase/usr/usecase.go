package usr

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rendau/kusec/internal/domain/usr/model"
	"github.com/rendau/kusec/internal/errs"
)

const (
	// passwordMinLen — минимальная длина пароля (в рунах).
	passwordMinLen = 8
	// passwordMaxLen — максимальная длина пароля в байтах: bcrypt молча обрезает
	// всё, что длиннее 72 байт, поэтому ограничиваем явно.
	passwordMaxLen = 72
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(svc ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
	}
}

func (u *Usecase) issueTokenPair(item *model.Main) (string, string, error) {
	access, err := u.sessionSvc.CreateToken(item.Id, item.IsAdmin, item.AppIds)
	if err != nil {
		return "", "", fmt.Errorf("sessionSvc.CreateToken: %w", err)
	}

	refresh, err := u.sessionSvc.CreateRefreshToken(item.Id, item.Password)
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

func (u *Usecase) Login(ctx context.Context, username, password, totpCode string) (*LoginResult, error) {
	username = strings.TrimSpace(username)

	item, found, err := u.svc.AuthByUsernamePassword(ctx, username, password)
	if err != nil {
		return nil, fmt.Errorf("svc.AuthByUsernamePassword: %w", err)
	}
	if !found || !item.Active {
		return nil, errs.NotAuthorized
	}

	if item.TotpEnabled {
		if strings.TrimSpace(totpCode) == "" {
			// пароль верный, но нужен второй фактор
			return &LoginResult{TotpRequired: true}, nil
		}
		if !u.svc.ValidateTotpCode(item.TotpSecret, totpCode) {
			return nil, errs.TotpInvalid
		}
		return u.issueLoginResult(item)
	}

	// 2FA не настроена: для админов привязка обязательна — выдаём enroll-токен
	// вместо полноценной сессии.
	if item.IsAdmin {
		setupToken, err := u.sessionSvc.CreateEnrollToken(item.Id)
		if err != nil {
			return nil, fmt.Errorf("sessionSvc.CreateEnrollToken: %w", err)
		}
		return &LoginResult{TotpSetupRequired: true, SetupToken: setupToken}, nil
	}

	return u.issueLoginResult(item)
}

// totpSetupUserId определяет пользователя для эндпоинтов настройки 2FA:
// это либо уже авторизованный пользователь (добровольная привязка), либо
// владелец валидного enroll-токена (обязательная привязка после логина).
func (u *Usecase) totpSetupUserId(ctx context.Context, setupToken string) (int64, error) {
	session := u.sessionSvc.FromContext(ctx)
	if session.IsAuthorized() {
		return session.Id, nil
	}
	setupToken = strings.TrimSpace(setupToken)
	if setupToken == "" {
		return 0, errs.NotAuthorized
	}
	usrId, err := u.sessionSvc.ParseEnrollToken(setupToken)
	if err != nil {
		return 0, errs.NotAuthorized
	}
	return usrId, nil
}

// EnrollTotp генерирует новый секрет и otpauth-URL для привязки 2FA.
func (u *Usecase) EnrollTotp(ctx context.Context, setupToken string) (string, string, error) {
	usrId, err := u.totpSetupUserId(ctx, setupToken)
	if err != nil {
		return "", "", err
	}
	secret, url, err := u.svc.EnrollTotp(ctx, usrId)
	if err != nil {
		return "", "", fmt.Errorf("svc.EnrollTotp: %w", err)
	}
	return secret, url, nil
}

// ConfirmTotp подтверждает привязку 2FA первым кодом и выдаёт пару токенов.
func (u *Usecase) ConfirmTotp(ctx context.Context, setupToken, code string) (*LoginResult, error) {
	usrId, err := u.totpSetupUserId(ctx, setupToken)
	if err != nil {
		return nil, err
	}
	item, err := u.svc.ConfirmTotp(ctx, usrId, code)
	if err != nil {
		return nil, fmt.Errorf("svc.ConfirmTotp: %w", err)
	}
	return u.issueLoginResult(item)
}

// DisableTotp отключает 2FA текущему пользователю (требуется действующий код).
func (u *Usecase) DisableTotp(ctx context.Context, code string) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	session := u.sessionSvc.FromContext(ctx)
	if err := u.svc.DisableTotp(ctx, session.Id, code); err != nil {
		return fmt.Errorf("svc.DisableTotp: %w", err)
	}
	return nil
}

// ResetTotp принудительно сбрасывает 2FA пользователю (только админ).
func (u *Usecase) ResetTotp(ctx context.Context, id int64) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if id == 0 {
		return errs.IdRequired
	}
	if err := u.svc.ResetTotp(ctx, id); err != nil {
		return fmt.Errorf("svc.ResetTotp: %w", err)
	}
	return nil
}

// RefreshToken обменивает валидный refresh-токен на новую пару токенов
// (refresh ротируется). Пользователь перечитывается из БД: деактивация или
// смена роли вступают в силу при ближайшем обновлении, а смена пароля
// инвалидирует ранее выданные refresh-токены (несовпадение отпечатка).
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

	if _, err = u.sessionSvc.ParseRefreshToken(refreshToken, item.Password); err != nil {
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

func (u *Usecase) UpdateProfile(ctx context.Context, req *UpdateProfileReq) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	session := u.sessionSvc.FromContext(ctx)
	if req == nil || (req.Name == nil && req.Username == nil && req.Password == nil) {
		return errs.InvalidRequest
	}

	item, _, err := u.svc.Get(ctx, session.Id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if !item.Active {
		return errs.NotAuthorized
	}

	edit := &model.Edit{
		Name:     req.Name,
		Username: req.Username,
		Password: req.Password,
	}
	if err = u.validateEdit(edit, false); err != nil {
		return err
	}

	if err = u.svc.Update(ctx, session.Id, edit); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}

	return nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
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

func (u *Usecase) Get(ctx context.Context, id int64) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
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
		if !session.IsAdmin() {
			return 0, errs.NoPermission
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

	newId, err := u.svc.Create(ctx, obj)
	if err != nil {
		return 0, fmt.Errorf("svc.Create: %w", err)
	}

	return newId, nil
}

func (u *Usecase) Update(ctx context.Context, id int64, obj *model.Edit) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if id == 0 {
		return errs.IdRequired
	}

	if err := u.validateEdit(obj, false); err != nil {
		return err
	}

	if obj.AppIds == nil {
		obj.AppIds = []string{}
	}

	if err := u.svc.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}

	return nil
}

func (u *Usecase) Delete(ctx context.Context, id int64) error {
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	if id == 0 {
		return errs.IdRequired
	}

	if err := u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}

	return nil
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
