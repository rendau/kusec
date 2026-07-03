package apikey

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mechta-market/kusec/internal/constant"
	"github.com/mechta-market/kusec/internal/domain/apikey/model"
	apikeyService "github.com/mechta-market/kusec/internal/domain/apikey/service"
	sessionModel "github.com/mechta-market/kusec/internal/domain/session/model"
	"github.com/mechta-market/kusec/internal/errs"
	"github.com/mechta-market/kusec/internal/util"
)

// touchInterval — минимальный интервал обновления last_used_at ключа,
// чтобы не писать в БД на каждый запрос.
const touchInterval = time.Minute

type Usecase struct {
	svc        ServiceI
	usrSvc     UsrServiceI
	sessionSvc SessionServiceI

	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

func New(svc ServiceI, usrSvc UsrServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		usrSvc:     usrSvc,
		sessionSvc: sessionSvc,
		lastTouch:  map[string]time.Time{},
	}
}

// ── CRUD ────────────────────────────────────────────────

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}
	if err := util.RequirePageSize(pars.ListParams, 0); err != nil {
		return nil, 0, err
	}

	// не-админ видит только свои ключи
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		pars.UsrId = new(u.sessionSvc.FromContext(ctx).Id)
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}
	return items, tCount, nil
}

// Create создаёт ключ и возвращает (id, значение ключа) — значение
// показывается только один раз, в БД хранится хэш.
func (u *Usecase) Create(ctx context.Context, name string, usrId *int64) (string, string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return "", "", errs.NotAuthorized
	}

	session := u.sessionSvc.FromContext(ctx)

	targetUsrId := session.Id
	if usrId != nil && *usrId != session.Id {
		// выпуск ключа другому пользователю — только админам
		if !session.IsAdmin() {
			return "", "", errs.NoPermission
		}
		targetUsrId = *usrId
	}

	usr, _, err := u.usrSvc.Get(ctx, targetUsrId, true)
	if err != nil {
		return "", "", fmt.Errorf("usrSvc.Get: %w", err)
	}
	if !usr.Active {
		return "", "", errs.InvalidRequest
	}

	key, hash, prefix, err := apikeyService.GenerateKey()
	if err != nil {
		return "", "", fmt.Errorf("apikeyService.GenerateKey: %w", err)
	}

	newId, err := u.svc.Create(ctx, &model.Edit{
		UsrId:     &targetUsrId,
		Active:    new(true),
		Name:      &name,
		KeyHash:   &hash,
		KeyPrefix: &prefix,
	})
	if err != nil {
		return "", "", fmt.Errorf("svc.Create: %w", err)
	}

	return newId, key, nil
}

func (u *Usecase) Update(ctx context.Context, id string, active *bool, name *string) error {
	if err := u.requireOwnership(ctx, id); err != nil {
		return err
	}

	err := u.svc.Update(ctx, id, &model.Edit{
		Active: active,
		Name:   name,
	})
	if err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}
	return nil
}

func (u *Usecase) Delete(ctx context.Context, id string) error {
	if err := u.requireOwnership(ctx, id); err != nil {
		return err
	}

	if err := u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}
	return nil
}

// requireOwnership: не-админ управляет только своими ключами.
func (u *Usecase) requireOwnership(ctx context.Context, id string) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	if id == "" {
		return errs.IdRequired
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}

	if !u.sessionSvc.CtxIsAdmin(ctx) && item.UsrId != u.sessionSvc.FromContext(ctx).Id {
		return errs.NoPermission
	}

	return nil
}

// ── Аутентификация по ключу ─────────────────────────────

// SessionFromKey строит сессию по API-ключу: ключ активен, владелец активен —
// сессия наследует права владельца. Используется session-интерсептором.
func (u *Usecase) SessionFromKey(ctx context.Context, key string) (*sessionModel.Session, error) {
	if !strings.HasPrefix(key, constant.ApiKeyPrefix) {
		return nil, errs.NotAuthorized
	}

	item, found, err := u.svc.GetByKeyHash(ctx, apikeyService.HashKey(key))
	if err != nil {
		return nil, fmt.Errorf("svc.GetByKeyHash: %w", err)
	}
	if !found || !item.Active {
		return nil, errs.NotAuthorized
	}

	usr, found, err := u.usrSvc.Get(ctx, item.UsrId, false)
	if err != nil {
		return nil, fmt.Errorf("usrSvc.Get: %w", err)
	}
	if !found || !usr.Active {
		return nil, errs.NotAuthorized
	}

	u.touchLastUsed(ctx, item.Id)

	return &sessionModel.Session{
		Id:     usr.Id,
		Admin:  usr.IsAdmin,
		AppIds: usr.AppIds,
	}, nil
}

// touchLastUsed обновляет last_used_at не чаще touchInterval; ошибки не
// прерывают аутентификацию.
func (u *Usecase) touchLastUsed(ctx context.Context, id string) {
	u.touchMu.Lock()
	last, ok := u.lastTouch[id]
	if ok && time.Since(last) < touchInterval {
		u.touchMu.Unlock()
		return
	}
	u.lastTouch[id] = time.Now()
	u.touchMu.Unlock()

	if err := u.svc.TouchLastUsed(ctx, id); err != nil {
		slog.Warn("apikey: touch last_used_at", "error", err, "id", id)
	}
}
