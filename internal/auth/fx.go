package auth

import (
	authconfig "github.com/smallbiznis/valora/internal/auth/config"
	"github.com/smallbiznis/valora/internal/auth/oauth"
	"github.com/smallbiznis/valora/internal/auth/repository"
	"github.com/smallbiznis/valora/internal/auth/service"
	"go.uber.org/fx"
)

var Module = fx.Module("auth.service",
	fx.Provide(repository.New),
	fx.Provide(service.New),
	fx.Provide(oauth.NewService),
	fx.Provide(authconfig.ParseAuthProvidersFromEnv),
	fx.Provide(authconfig.BuildAuthProviderRegistry),
	fx.Invoke(ensureAuthProviderRegistry),
)

func ensureAuthProviderRegistry(_ authconfig.AuthProviderRegistry) {}
