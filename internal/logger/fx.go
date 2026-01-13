package logger

import (
	"context"

	"github.com/smallbiznis/valora/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// NewFromConfig creates a zap logger from Config and replaces globals.
func NewFromConfig(appCfg config.Config) (*zap.Logger, error) {
	return New(appCfg.Logger.Level)
}

func registerHooks(lc fx.Lifecycle, log *zap.Logger) {
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			_ = ctx
			_ = log.Sync()
			return nil
		},
	})
}

// Module wires the global zap logger for the application.
var Module = fx.Module("logger",
	fx.Provide(
		NewFromConfig,
	),
	fx.Invoke(registerHooks),
)
