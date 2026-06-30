package usr

import (
	"context"

	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/domain/usr/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id int64, errNE bool) (*model.Main, bool, error)
	AuthByUsernamePassword(ctx context.Context, username, password string) (*model.Main, bool, error)
	HasAny(ctx context.Context) (bool, error)
	Create(ctx context.Context, obj *model.Edit) (int64, error)
	Update(ctx context.Context, id int64, obj *model.Edit) error
	Delete(ctx context.Context, id int64) error

	ValidateTotpCode(secret, code string) bool
	EnrollTotp(ctx context.Context, usrId int64) (secret, url string, err error)
	ConfirmTotp(ctx context.Context, usrId int64, code string) (*model.Main, error)
	DisableTotp(ctx context.Context, usrId int64, code string) error
	ResetTotp(ctx context.Context, usrId int64) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
	CreateToken(usrId int64, isAdmin bool, appIds []string) (string, error)
	CreateRefreshToken(usrId int64, passwordHash string) (string, error)
	ParseRefreshToken(tokenStr, currentPasswordHash string) (int64, error)
	RefreshTokenUserId(tokenStr string) (int64, error)
	CreateEnrollToken(usrId int64) (string, error)
	ParseEnrollToken(tokenStr string) (int64, error)
}
