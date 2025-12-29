package apikey

import (
	"github.com/smallbiznis/valora/internal/apikey/repository"
	"github.com/smallbiznis/valora/internal/apikey/service"
	"go.uber.org/fx"
)

var Module = fx.Module("apikey.service",
	fx.Provide(repository.Provide),
	fx.Provide(service.New),
)
