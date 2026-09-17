package audit

import (
	"context"

	appModel "github.com/rendau/kusec/internal/domain/app/model"
	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	configmapModel "github.com/rendau/kusec/internal/domain/configmap/model"
	secretModel "github.com/rendau/kusec/internal/domain/secret/model"
	sessionModel "github.com/rendau/kusec/internal/domain/session/model"
	usrModel "github.com/rendau/kusec/internal/domain/usr/model"
)

type AuditServiceI interface {
	Create(ctx context.Context, obj *auditModel.Main) error
}

type UsrServiceI interface {
	Get(ctx context.Context, id int64, errNE bool) (*usrModel.Main, bool, error)
}

type AppServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*appModel.Main, bool, error)
}

type SecretServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*secretModel.Main, bool, error)
}

type ConfigMapServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*configmapModel.Main, bool, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}
