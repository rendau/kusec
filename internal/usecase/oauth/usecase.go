// Package oauth — OAuth 2.1 сервер авторизации встроенного MCP-сервера: MCP-
// клиенты с коннекторами (приложения Claude) подключаются по адресу —
// пользователь входит в админку и разрешает клиента, а клиент получает
// токены, которые работают как mcp_only-ключ этого пользователя.
package oauth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rendau/kusec/internal/constant"
	commonModel "github.com/rendau/kusec/internal/domain/common/model"
	"github.com/rendau/kusec/internal/domain/oauth/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/errs"
)

const (
	// Scope — единственный scope: доступ к MCP-серверу.
	Scope = "mcp"

	codeTTL = 2 * time.Minute

	// touchInterval ограничивает запись last_used_at: MCP-клиент ходит на
	// каждый вызов инструмента.
	touchInterval = time.Minute
)

type Config struct {
	Enabled bool
	// Issuer — публичный адрес MCP-сервера: одновременно OAuth issuer и
	// защищаемый ресурс.
	Issuer string
	// AdminURL — публичный адрес админки, там страница согласия.
	AdminURL     string
	RedirectUris []string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

type Usecase struct {
	conf        Config
	svc         ServiceI
	clientCodec ClientCodecI
	usrSvc      UsrServiceI
	sessionSvc  SessionServiceI
	txm         TransactionManagerI
	auditRec    AuditRecorderI

	touchMu   sync.Mutex
	lastTouch map[string]time.Time
}

func New(
	conf Config,
	svc ServiceI,
	clientCodec ClientCodecI,
	usrSvc UsrServiceI,
	sessionSvc SessionServiceI,
	txm TransactionManagerI,
	auditRec AuditRecorderI,
) *Usecase {
	return &Usecase{
		conf:        conf,
		svc:         svc,
		clientCodec: clientCodec,
		usrSvc:      usrSvc,
		sessionSvc:  sessionSvc,
		txm:         txm,
		auditRec:    auditRec,
		lastTouch:   map[string]time.Time{},
	}
}

func (u *Usecase) Config() Config {
	return u.conf
}

// ListGrants — подключённые клиенты пользователя.
func (u *Usecase) ListGrants(ctx context.Context) ([]*model.Grant, error) {
	session, err := u.requireHuman(ctx)
	if err != nil {
		return nil, err
	}

	items, _, err := u.svc.ListGrants(ctx, &model.GrantListReq{
		ListParams: commonModel.ListParams{Sort: []string{"created_at"}},
		UsrId:      &session.Id,
		Connected:  new(true),
	})
	if err != nil {
		return nil, fmt.Errorf("svc.ListGrants: %w", err)
	}

	return items, nil
}

// DeleteGrant отключает клиента пользователя: его токены перестают работать
// сразу.
func (u *Usecase) DeleteGrant(ctx context.Context, id string) error {
	session, err := u.requireHuman(ctx)
	if err != nil {
		return err
	}
	if id == "" {
		return errs.IdRequired
	}

	item, _, err := u.svc.GetGrant(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.GetGrant: %w", err)
	}
	if item.UsrId != session.Id {
		return errs.NoPermission
	}

	return u.deleteGrant(ctx, item)
}

func (u *Usecase) deleteGrant(ctx context.Context, item *model.Grant) error {
	return u.txm.TxFn(ctx, func(ctx context.Context) error {
		if err := u.svc.DeleteGrant(ctx, item.Id); err != nil {
			return fmt.Errorf("svc.DeleteGrant: %w", err)
		}
		if err := u.auditRec.RecordOAuthGrant(ctx, item, nil, nil); err != nil {
			return fmt.Errorf("auditRec.RecordOAuthGrant: %w", err)
		}
		return nil
	})
}

// McpSessionFromToken аутентифицирует запрос MCP по access-токену. Как у
// api-ключей, сессия строится по живой записи пользователя, а подключение
// играет роль mcp_only-ключа (аудит, привязка MCP-сессии).
func (u *Usecase) McpSessionFromToken(ctx context.Context, token string) (*sessionModel.Session, error) {
	if !u.conf.Enabled {
		return nil, errs.NotAuthorized
	}

	item, found, err := u.svc.FindToken(ctx, token, model.TokenKindAccess)
	if err != nil {
		return nil, fmt.Errorf("svc.FindToken: %w", err)
	}
	if !found {
		u.logRejectedToken(ctx, token)
		return nil, errs.NotAuthorized
	}

	grant, found, err := u.svc.GetGrant(ctx, item.GrantId, false)
	if err != nil {
		return nil, fmt.Errorf("svc.GetGrant: %w", err)
	}
	if !found {
		slog.Warn("oauth: access token rejected", "reason", "connection removed", "grant_id", item.GrantId)
		return nil, errs.NotAuthorized
	}

	session, err := u.grantSession(ctx, grant)
	if err != nil {
		if errors.Is(err, errs.NotAuthorized) {
			slog.Warn("oauth: access token rejected", "reason", "user is not active",
				"grant_id", grant.Id, "usr_id", grant.UsrId)
		}
		return nil, err
	}

	u.touchLastUsed(ctx, grant.Id)

	return session, nil
}

// logRejectedToken объясняет, почему токен не живой: по access-логу не
// отличить устаревший токен от чужого.
func (u *Usecase) logRejectedToken(ctx context.Context, token string) {
	item, found, err := u.svc.FindAnyToken(ctx, token)
	switch {
	case err != nil:
		slog.Warn("oauth: access token rejected", "reason", "lookup failed", "error", err)
	case !found:
		slog.Warn("oauth: access token rejected", "reason", "unknown token")
	case item.Kind != model.TokenKindAccess:
		slog.Warn("oauth: access token rejected", "reason", "not an access token", "kind", item.Kind, "grant_id", item.GrantId)
	default:
		slog.Warn("oauth: access token rejected", "reason", "expired", "expires_at", item.ExpiresAt, "grant_id", item.GrantId)
	}
}

// grantSession — сессия владельца подключения с правами mcp_only-ключа;
// неактивный пользователь MCP не получает.
func (u *Usecase) grantSession(ctx context.Context, grant *model.Grant) (*sessionModel.Session, error) {
	usr, found, err := u.usrSvc.Get(ctx, grant.UsrId, false)
	if err != nil {
		return nil, fmt.Errorf("usrSvc.Get: %w", err)
	}
	if !found || !usr.Active {
		return nil, errs.NotAuthorized
	}

	return &sessionModel.Session{
		Id:         usr.Id,
		Admin:      usr.IsAdmin,
		AppIds:     usr.AppIds,
		Name:       usr.Name,
		Scope:      constant.ApiKeyScopeMcpOnly,
		Source:     constant.SourceMcp,
		ApiKeyId:   grant.Id,
		ApiKeyName: grantDisplayName(grant),
	}, nil
}

func grantDisplayName(grant *model.Grant) string {
	name := grant.ClientName
	if name == "" {
		name = defaultClientName
	}
	return "OAuth: " + name
}

// requireHuman — страница согласия и список подключений доступны только из
// входа в админку: сессия по api-ключу или OAuth-токену подключать клиентов
// не может.
func (u *Usecase) requireHuman(ctx context.Context) (*sessionModel.Session, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAuthorized() {
		return nil, errs.NotAuthorized
	}
	if session.ApiKeyId != "" {
		return nil, errs.NoPermission
	}
	return session, nil
}

func (u *Usecase) touchLastUsed(ctx context.Context, id string) {
	u.touchMu.Lock()
	if last, ok := u.lastTouch[id]; ok && time.Since(last) < touchInterval {
		u.touchMu.Unlock()
		return
	}
	u.lastTouch[id] = time.Now()
	u.touchMu.Unlock()

	if err := u.svc.TouchGrantLastUsed(ctx, id); err != nil {
		slog.Warn("oauth: touch last_used_at", "error", err, "id", id)
	}
}
