package cloudmetrics

import (
	"context"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/smallbiznis/railzway/internal/config"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var Module = fx.Module("cloud.metrics",
	fx.Provide(func() *prometheus.Registry {
		return prometheus.NewRegistry()
	}),
	fx.Provide(NewPusher),
	fx.Provide(func(cfg config.Config, pusher Pusher, logger *zap.Logger) *CloudMetrics {
		if !cfg.Cloud.Metrics.Enabled {
			return nil
		}
		return New(nil, pusher, cfg.InstanceID, cfg.AppVersion, logger)
	}),
	fx.Invoke(func(lc fx.Lifecycle, c *CloudMetrics, logger *zap.Logger, db *gorm.DB) {
		if c == nil {
			return
		}

		if logger == nil {
			logger = zap.NewNop()
		}

		ctx, cancel := context.WithCancel(context.Background())
		lc.Append(fx.Hook{
			OnStart: func(context.Context) error {
				logger.Info("starting cloud metrics background worker")
				go func() {
					ticker := time.NewTicker(30 * time.Minute)
					defer ticker.Stop()

					// Initial push
					updateSystemMetrics(c)
					updateOrganizationCount(ctx, c, db)
					if err := c.Push(ctx); err != nil {
						logger.Error("initial cloud metrics push failed", zap.Error(err))
					}

					for {
						select {
						case <-ticker.C:
							updateSystemMetrics(c)
							updateOrganizationCount(ctx, c, db)
							if err := c.Push(ctx); err != nil {
								logger.Error("periodic cloud metrics push failed", zap.Error(err))
							}
						case <-ctx.Done():
							logger.Info("stopping cloud metrics background worker")
							return
						}
					}
				}()
				return nil
			},
			OnStop: func(context.Context) error {
				cancel()
				return nil
			},
		})
	}),
)

func updateSystemMetrics(c *CloudMetrics) {
	if c == nil {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	c.SetMemoryUsage(m.Sys)
}

func updateOrganizationCount(ctx context.Context, c *CloudMetrics, db *gorm.DB) {
	if c == nil || db == nil {
		return
	}
	var count int64
	if err := db.WithContext(ctx).Table("organization").Count(&count).Error; err != nil {
		return
	}
	c.SetOrganizationsTotal(count)
}
