package scheduler

import (
	"context"

	"github.com/smallbiznis/valora/internal/config"
	"go.uber.org/fx"
)

var Module = fx.Module("scheduler",
	fx.Provide(DefaultConfig),
	fx.Provide(New),
	fx.Invoke(NewScheduler),
)

func NewScheduler(lc fx.Lifecycle, cfg config.Config, sched *Scheduler) {
	if cfg.IsCloud() {
		return
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ctx, cancel := context.WithCancel(context.Background())

			go sched.RunForever(ctx)

			lc.Append(fx.Hook{
				OnStop: func(context.Context) error {
					cancel()
					return nil
				},
			})

			return nil
		},
	})
}
