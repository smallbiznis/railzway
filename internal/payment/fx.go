package payment

import (
	"github.com/smallbiznis/valora/internal/payment/adapters"
	"github.com/smallbiznis/valora/internal/payment/adapters/stripe"
	"github.com/smallbiznis/valora/internal/payment/repository"
	"github.com/smallbiznis/valora/internal/payment/service"
	"go.uber.org/fx"
)

var Module = fx.Module("payment.service",
	fx.Provide(repository.Provide),
	fx.Provide(func() *adapters.Registry {
		return adapters.NewRegistry(stripe.NewFactory())
	}),
	fx.Provide(service.NewService),
)
