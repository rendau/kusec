// Package oauthcleanup — фоновая чистка OAuth-входа MCP: истёкшие токены и
// подключения, оставшиеся без токенов (брошенное согласие, истёкший refresh).
package oauthcleanup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	oauthModel "github.com/rendau/kusec/internal/domain/oauth/model"
)

const (
	// tickInterval — период чистки; первая выполняется сразу на старте.
	tickInterval = time.Hour

	// idleGrantTTL: подключение без токенов старше этого — брошенное согласие
	// или истёкшее подключение.
	idleGrantTTL = time.Hour
)

type OAuthServiceI interface {
	DeleteExpiredTokens(ctx context.Context) (int64, error)
	DeleteIdleGrants(ctx context.Context, createdBefore time.Time) ([]*oauthModel.Grant, error)
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

type AuditRecorderI interface {
	RecordOAuthGrant(ctx context.Context, old, cur *oauthModel.Grant, batchId *string) error
	NewBatchId() string
}

type Service struct {
	oauthSvc OAuthServiceI
	txm      TransactionManagerI
	auditRec AuditRecorderI

	wg sync.WaitGroup
}

func New(oauthSvc OAuthServiceI, txm TransactionManagerI, auditRec AuditRecorderI) *Service {
	return &Service{
		oauthSvc: oauthSvc,
		txm:      txm,
		auditRec: auditRec,
	}
}

// Start запускает периодическую чистку; останавливается отменой ctx.
func (s *Service) Start(ctx context.Context) {
	s.wg.Go(func() {
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()

		for {
			s.Cleanup(ctx)

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

func (s *Service) Wait() {
	s.wg.Wait()
}

// Cleanup удаляет истёкшие токены, затем подключения без токенов; удаление
// подключения пишется в аудит в той же транзакции.
func (s *Service) Cleanup(ctx context.Context) {
	tokens, err := s.oauthSvc.DeleteExpiredTokens(ctx)
	if err != nil {
		slog.Error("oauth: cleanup tokens", "error", err)
		return
	}

	var grants int
	err = s.txm.TxFn(ctx, func(ctx context.Context) error {
		items, err := s.oauthSvc.DeleteIdleGrants(ctx, time.Now().Add(-idleGrantTTL))
		if err != nil {
			return fmt.Errorf("oauthSvc.DeleteIdleGrants: %w", err)
		}
		grants = len(items)
		if grants == 0 {
			return nil
		}

		batchId := s.auditRec.NewBatchId()
		for _, item := range items {
			if err = s.auditRec.RecordOAuthGrant(ctx, item, nil, &batchId); err != nil {
				return fmt.Errorf("auditRec.RecordOAuthGrant: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("oauth: cleanup grants", "error", err)
		return
	}

	if grants > 0 {
		slog.Info("oauth: cleanup", "tokens", tokens, "grants", grants)
	}
}
