package logger

import (
	"context"
	"fmt"
	"strings"
	"time"

	obscontext "github.com/smallbiznis/valora/internal/observability/context"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config configures the zap logger.
type Config struct {
	ServiceName string
	Environment string
	Version     string
	Level       string
	Format      string
	Debug       bool

	SamplingInitial     int
	SamplingThereafter  int
	SamplingWindow      time.Duration
	IncludeCaller       bool
	IncludeStackOnError bool
}

// New builds a structured zap.Logger and registers lifecycle hooks.
func New(lc fx.Lifecycle, cfg Config) (*zap.Logger, error) {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Encoding = normalizeFormat(cfg.Format)
	zapCfg.EncoderConfig.TimeKey = "ts"
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.OutputPaths = []string{"stdout"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	level := strings.TrimSpace(cfg.Level)
	if level == "" {
		level = "info"
	}
	if err := zapCfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}

	options := []zap.Option{}
	if cfg.IncludeCaller {
		options = append(options, zap.AddCaller())
	}
	if cfg.IncludeStackOnError {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	initial := cfg.SamplingInitial
	thereafter := cfg.SamplingThereafter
	window := cfg.SamplingWindow
	if initial == 0 {
		initial = 100
	}
	if thereafter == 0 {
		thereafter = 100
	}
	if window == 0 {
		window = time.Second
	}

	options = append(options, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewSamplerWithOptions(core, window, initial, thereafter)
	}))

	logger, err := zapCfg.Build(options...)
	if err != nil {
		return nil, err
	}

	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "valora"
	}
	environment := strings.TrimSpace(cfg.Environment)
	version := strings.TrimSpace(cfg.Version)

	logger = logger.With(
		zap.String("service", serviceName),
		zap.String("env", environment),
		zap.String("version", version),
	)
	zap.ReplaceGlobals(logger)

	if lc != nil {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				_ = ctx
				_ = logger.Sync()
				return nil
			},
		})
	}

	return logger, nil
}

func normalizeFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "console" {
		return "console"
	}
	return "json"
}

// FromContext returns a logger enriched with request-scoped fields.
func FromContext(ctx context.Context) *zap.Logger {
	return WithContext(ctx, zap.L())
}

// WithContext enriches the provided logger with correlation fields.
func WithContext(ctx context.Context, base *zap.Logger) *zap.Logger {
	if ctx == nil {
		return base
	}

	requestID := obscontext.RequestIDFromContext(ctx)
	orgID := obscontext.OrgIDFromContext(ctx)
	actorType, actorID := obscontext.ActorFromContext(ctx)
	traceFields := traceFieldsFromContext(ctx)

	fields := []zap.Field{
		zap.String("request_id", requestID),
		zap.String("org_id", orgID),
		zap.String("actor_type", actorType),
		zap.String("actor_id", actorID),
	}
	fields = append(fields, traceFields...)

	return base.With(fields...)
}

// WithOrg adds organization identifier fields to the logger.
func WithOrg(log *zap.Logger, orgID string) *zap.Logger {
	if log == nil {
		return nil
	}
	return log.With(zap.String("org_id", strings.TrimSpace(orgID)))
}

// WithActor adds actor type and id fields to the logger.
func WithActor(log *zap.Logger, actorType, actorID string) *zap.Logger {
	if log == nil {
		return nil
	}
	return log.With(
		zap.String("actor_type", strings.TrimSpace(actorType)),
		zap.String("actor_id", strings.TrimSpace(actorID)),
	)
}

// WithRequest adds request and trace fields to the logger.
func WithRequest(log *zap.Logger, requestID, traceID, spanID string) *zap.Logger {
	if log == nil {
		return nil
	}
	return log.With(
		zap.String("request_id", strings.TrimSpace(requestID)),
		zap.String("trace_id", strings.TrimSpace(traceID)),
		zap.String("span_id", strings.TrimSpace(spanID)),
	)
}

func traceFieldsFromContext(ctx context.Context) []zap.Field {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return []zap.Field{
			zap.String("trace_id", ""),
			zap.String("span_id", ""),
		}
	}
	return []zap.Field{
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	}
}
