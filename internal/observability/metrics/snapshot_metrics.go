package metrics

import (
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type SnapshotMetrics struct {
	usageSnapshotLag           *prometheus.HistogramVec
	usageSnapshotBacklog       *prometheus.GaugeVec
	usageSnapshotBacklogAll    prometheus.Gauge
	usageSnapshotBacklogOldest *prometheus.GaugeVec
	usageSnapshotProcessed     *prometheus.CounterVec
}

var (
	snapshotMetricsOnce sync.Once
	snapshotMetrics     *SnapshotMetrics
)

func Snapshot() *SnapshotMetrics {
	return SnapshotWithConfig(Config{})
}

func SnapshotWithConfig(cfg Config) *SnapshotMetrics {
	snapshotMetricsOnce.Do(func() {
		snapshotMetrics = newSnapshotMetrics(prometheus.DefaultRegisterer, cfg)
	})
	return snapshotMetrics
}

func ResetSnapshotMetricsForTest() {
	snapshotMetricsOnce = sync.Once{}
	snapshotMetrics = nil
}

func newSnapshotMetrics(registerer prometheus.Registerer, cfg Config) *SnapshotMetrics {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "railzway"
	}
	environment := strings.TrimSpace(cfg.Environment)
	if environment == "" {
		environment = "unknown"
	}

	constLabels := prometheus.Labels{
		"service": serviceName,
		"env":     environment,
	}

	usageSnapshotLag := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "railzway_usage_snapshot_lag_seconds",
			Help: "Lag between usage recorded_at and snapshot time to measure billing SLA.",
			Buckets: []float64{
				60,     // 1m
				300,    // 5m
				900,    // 15m
				3600,   // 1h
				14400,  // 4h
				43200,  // 12h
				86400,  // 24h (SLA boundary)
				172800, // 48h
			},
			ConstLabels: constLabels,
		},
		[]string{"result"}, // success | late
	)

	usageSnapshotBacklog := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "railzway_usage_snapshot_backlog_total",
			Help:        "Number of usage events pending snapshot by status.",
			ConstLabels: constLabels,
		},
		[]string{"status"},
	)

	usageSnapshotProcessed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "railzway_usage_snapshot_processed_total",
			Help:        "Total usage snapshot records processed.",
			ConstLabels: constLabels,
		},
		[]string{"result"}, // success | skipped | failed
	)

	usageSnapshotBacklogAll := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name:        "railzway_usage_snapshot_backlog_all_total",
			Help:        "Total number of usage events pending snapshot.",
			ConstLabels: constLabels,
		},
	)

	usageSnapshotBacklogOldest := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "railzway_usage_snapshot_backlog_oldest_seconds",
			Help:        "Age of the oldest pending usage snapshot event.",
			ConstLabels: constLabels,
		},
		[]string{"status"},
	)

	registerer.MustRegister(
		usageSnapshotLag,
		usageSnapshotBacklog,
		usageSnapshotProcessed,
		usageSnapshotBacklogAll,
		usageSnapshotBacklogOldest,
	)

	return &SnapshotMetrics{
		usageSnapshotLag:           usageSnapshotLag,
		usageSnapshotBacklog:       usageSnapshotBacklog,
		usageSnapshotProcessed:     usageSnapshotProcessed,
		usageSnapshotBacklogAll:    usageSnapshotBacklogAll,
		usageSnapshotBacklogOldest: usageSnapshotBacklogOldest,
	}
}

func (m *SnapshotMetrics) ObserveSnapshotLag(lag time.Duration) {
	if m == nil {
		return
	}

	result := "success"
	if lag >= 24*time.Hour {
		result = "late"
	}

	m.usageSnapshotLag.WithLabelValues(result).Observe(lag.Seconds())
}

func (m *SnapshotMetrics) SetBacklog(status string, value int) {
	if m == nil {
		return
	}
	m.usageSnapshotBacklog.WithLabelValues(status).Set(float64(value))
}

func (m *SnapshotMetrics) IncSnapshotProcessed(result string) {
	if m == nil {
		return
	}
	m.usageSnapshotProcessed.WithLabelValues(result).Inc()
}

func (m *SnapshotMetrics) SetBacklogTotal(value int) {
	if m == nil {
		return
	}
	m.usageSnapshotBacklogAll.Set(float64(value))
}

func (m *SnapshotMetrics) SetBacklogOldest(status string, age time.Duration) {
	if m == nil {
		return
	}

	seconds := age.Seconds()
	if seconds < 0 {
		seconds = 0
	}

	m.usageSnapshotBacklogOldest.WithLabelValues(status).Set(seconds)
}
