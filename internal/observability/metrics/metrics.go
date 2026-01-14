package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Config configures the metrics provider.
type Config struct {
	Enabled          bool
	ExporterEndpoint string
	ExporterProtocol string
	ServiceName      string
	Environment      string
}

// Metrics exposes application-level instruments.
type Metrics struct {
	usageIngest      metric.Int64Counter
	paymentEvents    metric.Int64Counter
	ledgerEntries    metric.Int64Counter
	rateLimitAllowed metric.Int64Counter
	rateLimitDenied  metric.Int64Counter
}

// NewProvider configures and registers the meter provider.
func NewProvider(lc fx.Lifecycle, cfg Config, log *zap.Logger) (metric.MeterProvider, error) {
	if !cfg.Enabled {
		provider := noop.NewMeterProvider()
		otel.SetMeterProvider(provider)
		return provider, nil
	}

	exporter, err := newExporter(cfg.ExporterProtocol, cfg.ExporterEndpoint)
	if err != nil {
		return nil, err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second))
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	if lc != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				if log != nil {
					log.Info("shutting down meter provider")
				}
				return provider.Shutdown(ctx)
			},
		})
	}

	if log != nil {
		log.Info("metrics initialized",
			zap.String("endpoint", cfg.ExporterEndpoint),
			zap.String("protocol", cfg.ExporterProtocol),
		)
	}

	return provider, nil
}

// New configures the domain metrics instruments.
func New(cfg Config, provider metric.MeterProvider) (*Metrics, error) {
	name := strings.TrimSpace(cfg.ServiceName)
	if name == "" {
		name = "railzway"
	}
	meter := provider.Meter(name)

	usageIngest, err := meter.Int64Counter("railzway_usage_ingest_total")
	if err != nil {
		return nil, err
	}
	paymentEvents, err := meter.Int64Counter("railzway_payment_events_total")
	if err != nil {
		return nil, err
	}
	ledgerEntries, err := meter.Int64Counter("railzway_ledger_entries_total")
	if err != nil {
		return nil, err
	}
	rateLimitAllowed, err := meter.Int64Counter("railzway_rate_limit_allowed_total")
	if err != nil {
		return nil, err
	}
	rateLimitDenied, err := meter.Int64Counter("railzway_rate_limit_denied_total")
	if err != nil {
		return nil, err
	}

	return &Metrics{
		usageIngest:      usageIngest,
		paymentEvents:    paymentEvents,
		ledgerEntries:    ledgerEntries,
		rateLimitAllowed: rateLimitAllowed,
		rateLimitDenied:  rateLimitDenied,
	}, nil
}

// RecordUsageIngest increments usage ingest counts.
func (m *Metrics) RecordUsageIngest(ctx context.Context, meterCode string) {
	if m == nil {
		return
	}
	attrs := FilterAttributes(attribute.String("meter_code", strings.TrimSpace(meterCode)))
	m.usageIngest.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordPaymentEvent increments payment event counts.
func (m *Metrics) RecordPaymentEvent(ctx context.Context, provider, eventType string) {
	if m == nil {
		return
	}
	attrs := FilterAttributes(
		attribute.String("provider", strings.TrimSpace(provider)),
		attribute.String("event_type", strings.TrimSpace(eventType)),
	)
	m.paymentEvents.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordLedgerEntry increments ledger entry counts.
func (m *Metrics) RecordLedgerEntry(ctx context.Context, sourceType string) {
	if m == nil {
		return
	}
	attrs := FilterAttributes(attribute.String("source_type", strings.TrimSpace(sourceType)))
	m.ledgerEntries.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRateLimitAllowed increments rate limit allow counts.
func (m *Metrics) RecordRateLimitAllowed(ctx context.Context, orgID, endpoint string) {
	if m == nil {
		return
	}
	attrs := FilterAttributes(
		attribute.String("org_id", strings.TrimSpace(orgID)),
		attribute.String("endpoint", strings.TrimSpace(endpoint)),
	)
	m.rateLimitAllowed.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordRateLimitDenied increments rate limit deny counts.
func (m *Metrics) RecordRateLimitDenied(ctx context.Context, orgID, endpoint, reason string) {
	if m == nil {
		return
	}
	attrs := FilterAttributes(
		attribute.String("org_id", strings.TrimSpace(orgID)),
		attribute.String("endpoint", strings.TrimSpace(endpoint)),
		attribute.String("reason", strings.TrimSpace(reason)),
	)
	m.rateLimitDenied.Add(ctx, 1, metric.WithAttributes(attrs...))
}

func newExporter(protocol, endpoint string) (sdkmetric.Exporter, error) {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	switch protocol {
	case "http", "http/protobuf":
		opts := []otlpmetrichttp.Option{}
		if endpoint != "" {
			opts = append(opts, otlpmetrichttp.WithEndpoint(endpoint))
		}
		return otlpmetrichttp.New(context.Background(), opts...)
	case "grpc", "grpc/protobuf", "":
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithInsecure()}
		if endpoint != "" {
			opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))
		}
		return otlpmetricgrpc.New(context.Background(), opts...)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol %q", protocol)
	}
}

var allowedLabelKeys = map[attribute.Key]struct{}{
	"org_id":      {},
	"org_tier":    {},
	"endpoint":    {},
	"status_code": {},
	"meter_code":  {},
	"provider":    {},
	"event_type":  {},
	"source_type": {},
	"reason":      {},
}

// FilterAttributes strips disallowed labels to keep metrics low-cardinality.
func FilterAttributes(attrs ...attribute.KeyValue) []attribute.KeyValue {
	filtered := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		if _, ok := allowedLabelKeys[attr.Key]; !ok {
			continue
		}
		filtered = append(filtered, attr)
	}
	return filtered
}
