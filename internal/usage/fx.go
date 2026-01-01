package usage

import (
	"github.com/smallbiznis/valora/internal/cache"
	"github.com/smallbiznis/valora/internal/usage/service"
	"go.uber.org/fx"
)

var Module = fx.Module("usage.service",
	fx.Provide(cache.NewUsageResolverCache),
	fx.Provide(service.NewService),
)
