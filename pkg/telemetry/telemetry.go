package telemetry

import (
	"context"
	"time"

	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/pkg/telemetry/correlation"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module wires telemetry components via Fx.
var Module = fx.Options(
	fx.Provide(NewTracerProvider),
)

// NewTracerProvider configures OTLP exporter and tracer provider.
func NewTracerProvider(lc fx.Lifecycle, cfg config.Config, logger *zap.Logger) (*trace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		cancel()
		return nil, err
	}
	cancel()

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.AppName),
			attribute.String("service.version", cfg.AppVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSpanProcessor(&correlationSpanProcessor{}),
	)

	otel.SetTracerProvider(tp)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("shutting down tracer provider")
			return tp.Shutdown(ctx)
		},
	})

	logger.Info("telemetry initialized", zap.String("endpoint", cfg.OTLPEndpoint))
	return tp, nil
}

type correlationSpanProcessor struct{}

func (p *correlationSpanProcessor) OnStart(ctx context.Context, s trace.ReadWriteSpan) {
	_, cid := correlation.EnsureCorrelationID(ctx)
	s.SetAttributes(attribute.String("correlation_id", cid))
}

func (p *correlationSpanProcessor) OnEnd(trace.ReadOnlySpan) {}

func (p *correlationSpanProcessor) Shutdown(context.Context) error { return nil }

func (p *correlationSpanProcessor) ForceFlush(context.Context) error { return nil }
