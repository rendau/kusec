package audit

import (
	"github.com/prometheus/client_golang/prometheus"

	auditModel "github.com/rendau/kusec/internal/domain/audit/model"
	"github.com/rendau/kusec/internal/infra/metrics"
)

// Итоговое имя метрики получает префикс registry:
// <METRICS_NAMESPACE>_kusec_changes_total.
var metricChangesTotal *prometheus.CounterVec

func init() {
	metricChangesTotal = metrics.Factory.NewCounterVec(prometheus.CounterOpts{
		Name: "changes_total",
	}, []string{
		"entity_type",
		"action",
		"source",
	})
}

func observeChange(entry *auditModel.Main) {
	metricChangesTotal.WithLabelValues(entry.EntityType, entry.Action, entry.Source).Inc()
}
