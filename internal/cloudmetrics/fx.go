package cloudmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

var Module = fx.Module("cloud.metrics",
	fx.Provide(func() *prometheus.Registry {
		return prometheus.NewRegistry()
	}),
	fx.Provide(NewPusher),
	fx.Provide(New),
)
