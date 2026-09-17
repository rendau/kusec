package kube

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/rendau/kusec/internal/infra/metrics"
)

// Итоговые имена получают префикс registry: <METRICS_NAMESPACE>_kusec_*.
var (
	metricSyncRunsTotal    *prometheus.CounterVec
	metricSyncDurationSecs prometheus.Histogram
)

func init() {
	metricSyncRunsTotal = metrics.Factory.NewCounterVec(prometheus.CounterOpts{
		Name: "sync_runs_total",
	}, []string{
		"status",
	})

	metricSyncDurationSecs = metrics.Factory.NewHistogram(prometheus.HistogramOpts{
		Name: "sync_duration_seconds",
		Buckets: []float64{
			0.1,
			0.5,
			2,
			10,
			60,
		},
	})
}

func observeSyncRun(status string, duration time.Duration) {
	metricSyncRunsTotal.WithLabelValues(status).Inc()
	metricSyncDurationSecs.Observe(duration.Seconds())
}
