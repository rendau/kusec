package usr

import (
	"context"

	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	"github.com/rendau/kusec/internal/domain/usr/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id int64, errNE bool) (*model.Main, bool, error)
	GetByUsername(ctx context.Context, username string) (*model.Main, bool, error)
	PasswordMatches(passwordHash, password string) (bool, error)
	BurnPasswordCheck(password string)
	HasAny(ctx context.Context) (bool, error)
	Create(ctx context.Context, obj *model.Edit) (int64, error)
	Update(ctx context.Context, id int64, obj *model.Edit) error
	RevokeSessions(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error

	ValidateTotpCode(usrId int64, secret, code string) bool
	EnrollTotp(ctx context.Context, usrId int64) (secret, url string, err error)
	ConfirmTotp(ctx context.Context, usrId int64, code string) (*model.Main, error)
	DisableTotp(ctx context.Context, usrId int64, code string) error
	ResetTotp(ctx context.Context, usrId int64) error
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

type AuditRecorderI interface {
	RecordUsr(ctx context.Context, old, cur *model.Main, batchId *string) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
	CreateToken(usrId, ver int64) (string, error)
	CreateRefreshToken(usrId int64, passwordHash string, ver int64) (string, error)
	ParseAccessToken(tokenStr string) (usrId, ver int64, err error)
	ParseRefreshToken(tokenStr, currentPasswordHash string) (usrId, ver int64, err error)
	RefreshTokenUserId(tokenStr string) (int64, error)
}
