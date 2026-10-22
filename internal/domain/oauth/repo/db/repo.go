package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
	moboneTools "github.com/rendau/mobone/v2/tools"
	"github.com/samber/lo"

	commonRepoPg "github.com/rendau/kusec/internal/domain/common/repo/pg"
	domainModel "github.com/rendau/kusec/internal/domain/oauth/model"
	repoModel "github.com/rendau/kusec/internal/domain/oauth/repo/db/model"
)

var allowedSortFields = map[string]string{
	"created_at":   "created_at",
	"connected_at": "connected_at",
}

type Repo struct {
	*commonRepoPg.Base
	ModelStore *mobone.ModelStore
}

func New(con *pgxpool.Pool) *Repo {
	base := commonRepoPg.NewBase(con)
	return &Repo{
		Base: base,
		ModelStore: &mobone.ModelStore{
			Con:                base.Con,
			TransactionManager: base.TxM,
			QB:                 base.QB,
			TableName:          "oauth_grant",
		},
	}
}

func (r *Repo) ListGrants(ctx context.Context, pars *domainModel.GrantListReq) ([]*domainModel.Grant, int64, error) {
	conditions := map[string]any{}
	expressions := map[string][]any{}
	if pars.UsrId != nil {
		conditions["usr_id"] = *pars.UsrId
	}
	if pars.Connected != nil {
		if *pars.Connected {
			expressions["connected_at IS NOT NULL"] = nil
		} else {
			expressions["connected_at IS NULL"] = nil
		}
	}

	items := make([]*repoModel.GrantSelect, 0)

	totalCount, err := r.ModelStore.List(ctx, mobone.ListParams{
		Conditions:           conditions,
		ConditionExpressions: expressions,
		Page:                 pars.Page,
		PageSize:             pars.PageSize,
		WithTotalCount:       pars.WithTotalCount,
		OnlyCount:            pars.OnlyCount,
		Sort:                 moboneTools.ConstructSortColumns(allowedSortFields, pars.Sort),
	}, func(add bool) mobone.ListModelI {
		item := &repoModel.GrantSelect{}
		if add {
			items = append(items, item)
		}
		return item
	})
	if err != nil {
		return nil, 0, fmt.Errorf("ModelStore.List: %w", err)
	}

	return lo.Map(items, repoModel.EncodeGrantSelect), totalCount, nil
}

func (r *Repo) GetGrant(ctx context.Context, id string) (*domainModel.Grant, bool, error) {
	m := &repoModel.GrantSelect{PKId: id}

	found, err := r.ModelStore.Get(ctx, m)
	if err != nil {
		return nil, false, fmt.Errorf("ModelStore.Get: %w", err)
	}
	if !found {
		return nil, false, nil
	}

	return repoModel.EncodeGrantSelect(m, 0), true, nil
}

func (r *Repo) CreateGrant(ctx context.Context, obj *domainModel.Grant) (string, error) {
	m := repoModel.DecodeGrantUpsert(obj)

	if err := r.ModelStore.Create(ctx, m); err != nil {
		return "", fmt.Errorf("ModelStore.Create: %w", err)
	}

	return m.PKId, nil
}

func (r *Repo) DeleteGrant(ctx context.Context, id string) error {
	if err := r.ModelStore.Delete(ctx, &repoModel.GrantUpsert{PKId: id}); err != nil {
		return fmt.Errorf("ModelStore.Delete: %w", err)
	}

	return nil
}

func (r *Repo) MarkGrantConnected(ctx context.Context, id string) error {
	_, err := r.ModelStore.GetConnection(ctx).Exec(
		ctx,
		"UPDATE oauth_grant SET connected_at = COALESCE(connected_at, now()) WHERE id = $1",
		id,
	)
	if err != nil {
		return fmt.Errorf("Exec: %w", err)
	}

	return nil
}

func (r *Repo) TouchGrantLastUsed(ctx context.Context, id string) error {
	_, err := r.ModelStore.GetConnection(ctx).Exec(ctx, "UPDATE oauth_grant SET last_used_at = now() WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("Exec: %w", err)
	}

	return nil
}

func (r *Repo) CreateTokens(ctx context.Context, tokens []*domainModel.Token) error {
	qb := r.QB.Insert("oauth_token").Columns("token_hash", "grant_id", "kind", "expires_at")
	for _, t := range tokens {
		qb = qb.Values(t.Hash, t.GrantId, t.Kind, t.ExpiresAt)
	}

	query, args, err := qb.ToSql()
	if err != nil {
		return fmt.Errorf("ToSql: %w", err)
	}

	if _, err = r.ModelStore.GetConnection(ctx).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("Exec: %w", err)
	}

	return nil
}

// GetToken находит токен по хэшу, в том числе истёкший.
func (r *Repo) GetToken(ctx context.Context, hash string) (*domainModel.Token, bool, error) {
	result := &domainModel.Token{Hash: hash}

	err := r.ModelStore.GetConnection(ctx).QueryRow(
		ctx,
		"SELECT grant_id, kind, expires_at FROM oauth_token WHERE token_hash = $1",
		hash,
	).Scan(&result.GrantId, &result.Kind, &result.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("QueryRow: %w", err)
	}

	return result, true, nil
}

// TakeToken удаляет токен вида kind и возвращает его: коды и refresh-токены
// одноразовые, два одновременных обмена не могут оба выиграть.
func (r *Repo) TakeToken(ctx context.Context, hash, kind string) (*domainModel.Token, bool, error) {
	result := &domainModel.Token{Hash: hash, Kind: kind}

	err := r.ModelStore.GetConnection(ctx).QueryRow(
		ctx,
		"DELETE FROM oauth_token WHERE token_hash = $1 AND kind = $2 RETURNING grant_id, expires_at",
		hash, kind,
	).Scan(&result.GrantId, &result.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("QueryRow: %w", err)
	}

	return result, true, nil
}

func (r *Repo) DeleteExpiredTokens(ctx context.Context) (int64, error) {
	tag, err := r.ModelStore.GetConnection(ctx).Exec(ctx, "DELETE FROM oauth_token WHERE expires_at < now()")
	if err != nil {
		return 0, fmt.Errorf("Exec: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteIdleGrants удаляет подключения без токенов, созданные раньше
// createdBefore, и возвращает их (для аудита).
func (r *Repo) DeleteIdleGrants(ctx context.Context, createdBefore time.Time) ([]*domainModel.Grant, error) {
	rows, err := r.ModelStore.GetConnection(ctx).Query(
		ctx,
		`DELETE FROM oauth_grant g
		 WHERE g.created_at < $1
		   AND NOT EXISTS (SELECT 1 FROM oauth_token t WHERE t.grant_id = g.id)
		 RETURNING g.id, g.usr_id, g.client_id, g.client_name, g.redirect_uri, g.created_at, g.connected_at, g.last_used_at`,
		createdBefore,
	)
	if err != nil {
		return nil, fmt.Errorf("Query: %w", err)
	}
	defer rows.Close()

	result := make([]*domainModel.Grant, 0)
	for rows.Next() {
		item := &domainModel.Grant{}
		err = rows.Scan(&item.Id, &item.UsrId, &item.ClientId, &item.ClientName, &item.RedirectUri,
			&item.CreatedAt, &item.ConnectedAt, &item.LastUsedAt)
		if err != nil {
			return nil, fmt.Errorf("rows.Scan: %w", err)
		}
		result = append(result, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err: %w", err)
	}

	return result, nil
}
