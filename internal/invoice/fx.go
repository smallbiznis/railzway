package invoice

import (
	"github.com/smallbiznis/valora/internal/invoice/render"
	"github.com/smallbiznis/valora/internal/invoice/service"
	"go.uber.org/fx"
)

var Module = fx.Module("invoice.service",
	fx.Provide(render.NewRenderer),
	fx.Provide(service.NewService),
)
