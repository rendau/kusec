package service

import (
	"context"
	"time"

	"github.com/rendau/kusec/internal/domain/oauth/model"
)

type RepoDbI interface {
	ListGrants(ctx context.Context, pars *model.GrantListReq) ([]*model.Grant, int64, error)
	GetGrant(ctx context.Context, id string) (*model.Grant, bool, error)
	CreateGrant(ctx context.Context, obj *model.Grant) (string, error)
	DeleteGrant(ctx context.Context, id string) error
	MarkGrantConnected(ctx context.Context, id string) error
	TouchGrantLastUsed(ctx context.Context, id string) error

	CreateTokens(ctx context.Context, tokens []*model.Token) error
	GetToken(ctx context.Context, hash string) (*model.Token, bool, error)
	TakeToken(ctx context.Context, hash, kind string) (*model.Token, bool, error)

	DeleteExpiredTokens(ctx context.Context) (int64, error)
	DeleteIdleGrants(ctx context.Context, createdBefore time.Time) ([]*model.Grant, error)
}
