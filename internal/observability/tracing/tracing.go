package tracing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Config configures the tracer provider and exporter.
type Config struct {
	Enabled          bool
	ServiceName      string
	ServiceVersion   string
	Environment      string
	ExporterEndpoint string
	ExporterProtocol string
	SamplingRatio    float64
}

// NewProvider configures an OpenTelemetry tracer provider.
func NewProvider(lc fx.Lifecycle, cfg Config, log *zap.Logger) (*sdktrace.TracerProvider, error) {
	SetPropagator()
	if !cfg.Enabled {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return nil, nil
	}

	endpoint := strings.TrimSpace(cfg.ExporterEndpoint)
	protocol := strings.ToLower(strings.TrimSpace(cfg.ExporterProtocol))

	exporter, err := newExporter(protocol, endpoint)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	samplingRatio := clampRatio(cfg.SamplingRatio)
	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRatio))

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(provider)

	if lc != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				if log != nil {
					log.Info("shutting down tracer provider")
				}
				return provider.Shutdown(ctx)
			},
		})
	}

	if log != nil {
		log.Info("tracing initialized",
			zap.String("endpoint", endpoint),
			zap.String("protocol", protocol),
			zap.Float64("sampling_ratio", samplingRatio),
		)
	}

	return provider, nil
}

func newExporter(protocol, endpoint string) (sdktrace.SpanExporter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch protocol {
	case "http", "http/protobuf":
		opts := []otlptracehttp.Option{}
		if endpoint != "" {
			opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
		}
		return otlptracehttp.New(ctx, opts...)
	case "grpc", "grpc/protobuf", "":
		opts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure()}
		if endpoint != "" {
			opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
		}
		return otlptracegrpc.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol %q", protocol)
	}
}

func clampRatio(value float64) float64 {
	if value <= 0 {
		return 0.1
	}
	if value > 1 {
		return 1
	}
	return value
}
