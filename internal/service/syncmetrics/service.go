// Package syncmetrics — фоновое обновление Prometheus-гейджей состояния
// синхронизации: last_sync_timestamp_seconds и unsynced_objects по каждому
// приложению. Обновляется периодическим тиком (метрики нужны и когда sync
// давно не запускался — «изменено, но не применено» растёт само по себе).
package syncmetrics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	syncrunModel "github.com/rendau/kusec/internal/domain/syncrun/model"
	"github.com/rendau/kusec/internal/infra/metrics"
)

// refreshInterval — период пересчёта гейджей.
const refreshInterval = time.Minute

// Итоговые имена получают префикс registry: <METRICS_NAMESPACE>_kusec_*.
var (
	metricLastSyncTimestamp *prometheus.GaugeVec
	metricUnsyncedObjects   *prometheus.GaugeVec
)

func init() {
	metricLastSyncTimestamp = metrics.Factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "last_sync_timestamp_seconds",
	}, []string{
		"namespace",
		"app",
	})

	metricUnsyncedObjects = metrics.Factory.NewGaugeVec(prometheus.GaugeOpts{
		Name: "unsynced_objects",
	}, []string{
		"namespace",
		"app",
	})
}

type SyncRunServiceI interface {
	AppSyncStats(ctx context.Context) ([]*syncrunModel.AppSyncStat, error)
}

type Service struct {
	syncRunSvc SyncRunServiceI

	wg sync.WaitGroup
}

func New(syncRunSvc SyncRunServiceI) *Service {
	return &Service{syncRunSvc: syncRunSvc}
}

// Start запускает периодический пересчёт; останавливается отменой ctx.
// Без включённых метрик не запускается.
func (s *Service) Start(ctx context.Context) {
	if !metrics.Enabled {
		return
	}

	s.wg.Go(func() {
		s.refresh(ctx)

		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refresh(ctx)
			}
		}
	})
}

func (s *Service) Wait() {
	s.wg.Wait()
}

func (s *Service) refresh(ctx context.Context) {
	stats, err := s.syncRunSvc.AppSyncStats(ctx)
	if err != nil {
		slog.Warn("syncmetrics: refresh", "error", err)
		return
	}

	// Reset убирает метки удалённых/переименованных приложений.
	metricLastSyncTimestamp.Reset()
	metricUnsyncedObjects.Reset()

	for _, stat := range stats {
		if stat.LastSyncedAt != nil {
			metricLastSyncTimestamp.WithLabelValues(stat.Namespace, stat.AppSlug).
				Set(float64(stat.LastSyncedAt.Unix()))
		}
		metricUnsyncedObjects.WithLabelValues(stat.Namespace, stat.AppSlug).
			Set(float64(stat.UnsyncedCount))
	}
}
