package apikey

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rendau/kusec/internal/constant"
	"github.com/rendau/kusec/internal/domain/apikey/model"
	apikeyService "github.com/rendau/kusec/internal/domain/apikey/service"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/errs"
	"github.com/rendau/kusec/internal/util"
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
// показывается только один раз, в БД хранится хэш. mcpOnly-ключи принимает
// только MCP-эндпоинт.
func (u *Usecase) Create(ctx context.Context, name string, usrId *int64, mcpOnly bool) (string, string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return "", "", errs.NotAuthorized
	}

	session := u.sessionSvc.FromContext(ctx)

	// ключи с полным доступом к API выпускают только админы
	if !mcpOnly && !session.IsAdmin() {
		return "", "", errs.NoPermission
	}

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
		McpOnly:   &mcpOnly,
		Name:      &name,
		KeyHash:   &hash,
		KeyPrefix: &prefix,
	})
	if err != nil {
		return "", "", fmt.Errorf("svc.Create: %w", err)
	}

	return newId, key, nil
}

func (u *Usecase) Update(ctx context.Context, id string, active *bool, name *string, mcpOnly *bool) error {
	item, err := u.requireOwnership(ctx, id)
	if err != nil {
		return err
	}

	// снять ограничение mcp_only с ключа может только админ
	if mcpOnly != nil && !*mcpOnly && item.McpOnly && !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}

	err = u.svc.Update(ctx, id, &model.Edit{
		Active:  active,
		McpOnly: mcpOnly,
		Name:    name,
	})
	if err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}
	return nil
}

func (u *Usecase) Delete(ctx context.Context, id string) error {
	if _, err := u.requireOwnership(ctx, id); err != nil {
		return err
	}

	if err := u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}
	return nil
}

// requireOwnership: не-админ управляет только своими ключами.
func (u *Usecase) requireOwnership(ctx context.Context, id string) (*model.Main, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if id == "" {
		return nil, errs.IdRequired
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	if !u.sessionSvc.CtxIsAdmin(ctx) && item.UsrId != u.sessionSvc.FromContext(ctx).Id {
		return nil, errs.NoPermission
	}

	return item, nil
}

// ── Аутентификация по ключу ─────────────────────────────

// SessionFromKey строит сессию по API-ключу для основного API: mcp_only-ключи
// здесь отвергаются. Используется session-интерсептором.
func (u *Usecase) SessionFromKey(ctx context.Context, key string) (*sessionModel.Session, error) {
	return u.sessionFromKey(ctx, key, false)
}

// McpSessionFromKey строит сессию по API-ключу для встроенного MCP-эндпоинта:
// принимаются и обычные, и mcp_only-ключи.
func (u *Usecase) McpSessionFromKey(ctx context.Context, key string) (*sessionModel.Session, error) {
	return u.sessionFromKey(ctx, key, true)
}

// sessionFromKey: ключ активен, владелец активен — сессия наследует права владельца.
func (u *Usecase) sessionFromKey(ctx context.Context, key string, allowMcpOnly bool) (*sessionModel.Session, error) {
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
	if item.McpOnly && !allowMcpOnly {
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
