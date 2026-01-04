package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	"go.uber.org/zap"
)

func TestRunJobTimeoutDoesNotReturnErrorAndIncrementsTimeout(t *testing.T) {
	registry := prometheus.NewRegistry()
	restore := swapPrometheusRegistry(registry)
	defer restore()

	obsmetrics.ResetSchedulerMetricsForTest()
	obsmetrics.SchedulerWithConfig(obsmetrics.Config{
		ServiceName: "valora",
		Environment: "test",
	})

	node, err := snowflake.NewNode(1)
	if err != nil {
		t.Fatalf("snowflake node: %v", err)
	}

	s := &Scheduler{log: zap.NewNop(), genID: node}
	err = s.runJob(context.Background(), "timeout_job", 0, 5*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	labels := map[string]string{
		"service": "valora",
		"env":     "test",
		"job":     "timeout_job",
	}
	if got := getCounterValue(t, registry, "valora_scheduler_job_timeouts_total", labels); got != 1 {
		t.Fatalf("expected timeout count 1, got %v", got)
	}

	errorLabels := map[string]string{
		"service": "valora",
		"env":     "test",
		"job":     "timeout_job",
		"reason":  obsmetrics.SchedulerJobReasonDeadlineExceeded,
	}
	if got := getCounterValue(t, registry, "valora_scheduler_job_errors_total", errorLabels); got != 1 {
		t.Fatalf("expected error count 1, got %v", got)
	}
}

func swapPrometheusRegistry(registry *prometheus.Registry) func() {
	oldRegisterer := prometheus.DefaultRegisterer
	oldGatherer := prometheus.DefaultGatherer
	prometheus.DefaultRegisterer = registry
	prometheus.DefaultGatherer = registry
	return func() {
		prometheus.DefaultRegisterer = oldRegisterer
		prometheus.DefaultGatherer = oldGatherer
		obsmetrics.ResetSchedulerMetricsForTest()
	}
}

func getCounterValue(t *testing.T, registry *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, mf := range metricFamilies {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.Metric {
			if !labelsMatch(metric, labels) {
				continue
			}
			if metric.Counter == nil {
				t.Fatalf("metric %s is not a counter", name)
			}
			return metric.GetCounter().GetValue()
		}
	}
	t.Fatalf("metric %s with labels %v not found", name, labels)
	return 0
}

func labelsMatch(metric *dto.Metric, labels map[string]string) bool {
	if len(metric.Label) != len(labels) {
		return false
	}
	for _, label := range metric.Label {
		if labels[label.GetName()] != label.GetValue() {
			return false
		}
	}
	return true
}
