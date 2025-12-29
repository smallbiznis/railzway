package rating

import (
	"github.com/smallbiznis/valora/internal/rating/service"
	"go.uber.org/fx"
)

var Module = fx.Module("rating.service",
	fx.Provide(service.NewService),
)
