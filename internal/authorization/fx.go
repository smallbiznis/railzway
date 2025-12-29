package authorization

import "go.uber.org/fx"

var Module = fx.Module("authorization.service",
	fx.Provide(NewEnforcer),
	fx.Provide(NewService),
)
