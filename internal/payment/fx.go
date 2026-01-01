package payment

import (
	"github.com/smallbiznis/valora/internal/payment/adapters"
	"github.com/smallbiznis/valora/internal/payment/adapters/stripe"
	disputerepo "github.com/smallbiznis/valora/internal/payment/dispute/repository"
	disputeservice "github.com/smallbiznis/valora/internal/payment/dispute/service"
	"github.com/smallbiznis/valora/internal/payment/repository"
	paymentservice "github.com/smallbiznis/valora/internal/payment/service"
	"github.com/smallbiznis/valora/internal/payment/webhook"
	"go.uber.org/fx"
)

var Module = fx.Module("payment.service",
	fx.Provide(repository.Provide),
	fx.Provide(disputerepo.Provide),
	fx.Provide(func() *adapters.Registry {
		return adapters.NewRegistry(stripe.NewFactory())
	}),
	fx.Provide(paymentservice.NewService),
	fx.Provide(disputeservice.NewService),
	fx.Provide(webhook.NewService),
)
