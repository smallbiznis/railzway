package billingoverview

import (
	"github.com/smallbiznis/valora/internal/billingoverview/service"
	"go.uber.org/fx"
)

var Module = fx.Module("billingoverview.service",
	fx.Provide(service.NewService),
)
