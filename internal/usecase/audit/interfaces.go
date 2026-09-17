package audit

import (
	"context"

	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *auditModel.ListReq) ([]*auditModel.Main, int64, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
}
