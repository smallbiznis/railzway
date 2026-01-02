package snapshot

import (
	"context"

	"go.uber.org/fx"
)

var Module = fx.Module("usage.snapshot",
	fx.Provide(DefaultConfig),
	fx.Provide(NewWorker),
	fx.Invoke(runWorker),
)

func runWorker(lc fx.Lifecycle, worker *Worker) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go worker.RunForever(ctx)
			return nil
		},
	})
}
