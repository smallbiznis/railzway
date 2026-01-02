package usage

import (
	"github.com/smallbiznis/valora/internal/cache"
	"github.com/smallbiznis/valora/internal/usage/repository"
	"github.com/smallbiznis/valora/internal/usage/service"
	"github.com/smallbiznis/valora/internal/usage/snapshot"
	"go.uber.org/fx"
)

var Module = fx.Module("usage.service",
	fx.Provide(cache.NewUsageResolverCache),
	fx.Provide(repository.ProvideSnapshot),
	fx.Provide(service.NewService),
	snapshot.Module,
)
