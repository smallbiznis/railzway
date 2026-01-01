package billingdashboard

import (
	"github.com/smallbiznis/valora/internal/billingdashboard/rollup"
	"github.com/smallbiznis/valora/internal/billingdashboard/service"
	"go.uber.org/fx"
)

var Module = fx.Module("billingdashboard.service",
	fx.Provide(service.NewService),
	fx.Provide(rollup.NewService),
)
