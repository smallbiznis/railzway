package cloudmetrics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/prompb"
	"github.com/smallbiznis/valora/internal/config"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

const (
	exporterPrometheusRemoteWrite = "prometheus_remote_write"
	exporterPrometheusPushgateway = "prometheus_pushgateway"
	defaultPushTimeout            = 5 * time.Second
)

// Pusher sends Cloud accounting metrics from OSS to Valora Cloud.
// Implementations must not start background goroutines or expose /metrics.
type Pusher interface {
	Push(ctx context.Context, registry *prometheus.Registry) error
}

// NewPusher builds a pusher from config. Errors are logged and return nil to avoid blocking OSS workflows.
func NewPusher(cfg config.Config, logger *zap.Logger) Pusher {
	if logger == nil {
		logger = zap.NewNop()
	}
	if !cfg.IsCloud() || !cfg.Cloud.Metrics.Enabled {
		return nil
	}

	exporter := strings.ToLower(strings.TrimSpace(cfg.Cloud.Metrics.Exporter))
	endpoint := strings.TrimSpace(cfg.Cloud.Metrics.Endpoint)
	authToken := strings.TrimSpace(cfg.Cloud.Metrics.AuthToken)

	if exporter == "" {
		logger.Warn("cloud metrics disabled", zap.Error(errors.New("cloud.metrics.exporter is required")))
		return nil
	}
	if endpoint == "" {
		logger.Warn("cloud metrics disabled", zap.Error(errors.New("cloud.metrics.endpoint is required")))
		return nil
	}

	switch exporter {
	case exporterPrometheusRemoteWrite:
		if _, err := url.ParseRequestURI(endpoint); err != nil {
			logger.Warn("cloud metrics disabled", zap.Error(fmt.Errorf("invalid cloud.metrics.endpoint: %w", err)))
			return nil
		}
		return NewRemoteWritePusher(endpoint, authToken)
	case exporterPrometheusPushgateway:
		return NewPushgatewayPusher(endpoint, cfg.AppName, map[string]string{
			"environment": strings.TrimSpace(cfg.Environment),
		})
	default:
		logger.Warn("cloud metrics disabled", zap.String("exporter", exporter))
		return nil
	}
}

// RemoteWritePusher sends metrics to a Prometheus remote_write endpoint.
type RemoteWritePusher struct {
	endpoint   string
	authToken  string
	httpClient *http.Client
}

// NewRemoteWritePusher returns a pusher for Prometheus remote_write.
func NewRemoteWritePusher(endpoint, authToken string) *RemoteWritePusher {
	return &RemoteWritePusher{
		endpoint:   endpoint,
		authToken: strings.TrimSpace(authToken),
		httpClient: &http.Client{
			Timeout: defaultPushTimeout,
		},
	}
}

// Push sends the current registry metrics via remote_write.
func (p *RemoteWritePusher) Push(ctx context.Context, registry *prometheus.Registry) error {
	if p == nil || registry == nil {
		return nil
	}

	families, err := registry.Gather()
	if err != nil {
		return err
	}
	if len(families) == 0 {
		return nil
	}

	series := buildRemoteWriteSeries(families, time.Now().UnixMilli())
	if len(series) == 0 {
		return nil
	}

	req := &prompb.WriteRequest{Timeseries: series}
	payload, err := proto.Marshal(protoadapt.MessageV2Of(req))
	if err != nil {
		return err
	}

	compressed := snappy.Encode(nil, payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(compressed))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	if p.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.authToken)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("remote write returned %s", resp.Status)
	}
	return nil
}

// PushgatewayPusher sends metrics to a Prometheus Pushgateway.
type PushgatewayPusher struct {
	endpoint string
	job      string
	grouping map[string]string
}

// NewPushgatewayPusher returns a pusher for Prometheus Pushgateway.
func NewPushgatewayPusher(endpoint, job string, grouping map[string]string) *PushgatewayPusher {
	return &PushgatewayPusher{
		endpoint: endpoint,
		job:      strings.TrimSpace(job),
		grouping: grouping,
	}
}

// Push sends the current registry metrics to the Pushgateway.
func (p *PushgatewayPusher) Push(ctx context.Context, registry *prometheus.Registry) error {
	if p == nil || registry == nil {
		return nil
	}
	if strings.TrimSpace(p.endpoint) == "" {
		return errors.New("pushgateway endpoint is required")
	}
	if p.job == "" {
		return errors.New("pushgateway job is required")
	}

	pusher := push.New(p.endpoint, p.job).Gatherer(registry)
	for key, value := range p.grouping {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		pusher = pusher.Grouping(key, value)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	return pusher.PushContext(ctx)
}

func buildRemoteWriteSeries(families []*dto.MetricFamily, timestampMs int64) []prompb.TimeSeries {
	series := make([]prompb.TimeSeries, 0, len(families))
	for _, family := range families {
		switch family.GetType() {
		case dto.MetricType_COUNTER, dto.MetricType_GAUGE:
		default:
			continue
		}
		for _, metric := range family.GetMetric() {
			value := extractMetricValue(family.GetType(), metric)
			if value == nil {
				continue
			}
			labels := make([]prompb.Label, 0, len(metric.GetLabel())+1)
			labels = append(labels, prompb.Label{Name: "__name__", Value: family.GetName()})
			for _, label := range metric.GetLabel() {
				labels = append(labels, prompb.Label{Name: label.GetName(), Value: label.GetValue()})
			}
			sort.Slice(labels, func(i, j int) bool {
				return labels[i].Name < labels[j].Name
			})

			series = append(series, prompb.TimeSeries{
				Labels: labels,
				Samples: []prompb.Sample{{
					Value:     *value,
					Timestamp: timestampMs,
				}},
			})
		}
	}
	return series
}

func extractMetricValue(metricType dto.MetricType, metric *dto.Metric) *float64 {
	if metric == nil {
		return nil
	}
	switch metricType {
	case dto.MetricType_COUNTER:
		if metric.GetCounter() == nil {
			return nil
		}
		value := metric.GetCounter().GetValue()
		return &value
	case dto.MetricType_GAUGE:
		if metric.GetGauge() == nil {
			return nil
		}
		value := metric.GetGauge().GetValue()
		return &value
	default:
		return nil
	}
}
