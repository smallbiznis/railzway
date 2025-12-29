package billingcycle

import (
	"github.com/smallbiznis/valora/internal/billingcycle/service"
	"go.uber.org/fx"
)

var Module = fx.Module("billingcycle.service",
	fx.Provide(service.NewService),
)
