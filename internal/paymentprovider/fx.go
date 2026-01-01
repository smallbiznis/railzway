package paymentprovider

import (
	"github.com/smallbiznis/valora/internal/paymentprovider/repository"
	"github.com/smallbiznis/valora/internal/paymentprovider/service"
	"go.uber.org/fx"
)

var Module = fx.Module("paymentprovider.service",
	fx.Provide(repository.Provide),
	fx.Provide(service.New),
)
