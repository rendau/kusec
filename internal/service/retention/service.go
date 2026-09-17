// Package retention — фоновая чистка аудита и журнала sync по сроку
// хранения (config AUDIT_RETENTION_DAYS). Единственный способ удаления
// записей аудита: через API они не удаляются и не редактируются.
package retention

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// tickInterval — период проверки; первая чистка выполняется сразу на старте.
const tickInterval = time.Hour

type AuditServiceI interface {
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

type SyncRunServiceI interface {
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}

type Service struct {
	auditSvc      AuditServiceI
	syncRunSvc    SyncRunServiceI
	retentionDays int

	wg sync.WaitGroup
}

func New(auditSvc AuditServiceI, syncRunSvc SyncRunServiceI, retentionDays int) *Service {
	return &Service{
		auditSvc:      auditSvc,
		syncRunSvc:    syncRunSvc,
		retentionDays: retentionDays,
	}
}

// Start запускает периодическую чистку; останавливается отменой ctx.
// retentionDays <= 0 — чистка выключена.
func (s *Service) Start(ctx context.Context) {
	if s.retentionDays <= 0 {
		slog.Warn("retention: disabled (AUDIT_RETENTION_DAYS <= 0)")
		return
	}

	s.wg.Go(func() {
		s.cleanup(ctx)

		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanup(ctx)
			}
		}
	})
}

func (s *Service) Wait() {
	s.wg.Wait()
}

func (s *Service) cleanup(ctx context.Context) {
	before := time.Now().AddDate(0, 0, -s.retentionDays)

	auditCount, err := s.auditSvc.DeleteOlderThan(ctx, before)
	if err != nil {
		slog.Error("retention: cleanup audit", "error", err)
	}

	syncRunCount, err := s.syncRunSvc.DeleteOlderThan(ctx, before)
	if err != nil {
		slog.Error("retention: cleanup sync_run", "error", err)
	}

	if auditCount > 0 || syncRunCount > 0 {
		slog.Info("retention: cleaned up",
			"audit", auditCount,
			"sync_run", syncRunCount,
			"before", before.Format(time.RFC3339),
		)
	}
}
