// @title           Railzway API
// @version         1.0
// @description     Railzway Billing & Operations API
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@railzway.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api
// @Schemes 	http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

package main

import (
	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/railzway/internal/apikey"
	"github.com/smallbiznis/railzway/internal/auth"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/meter"
	"github.com/smallbiznis/railzway/internal/observability"
	"github.com/smallbiznis/railzway/internal/ratelimit"
	"github.com/smallbiznis/railzway/internal/server"
	"github.com/smallbiznis/railzway/internal/usage"
	"github.com/smallbiznis/railzway/pkg/db"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		config.Module,
		observability.Module,
		fx.Provide(RegisterSnowflake),
		db.Module,
		clock.Module,

		// Core dependencies for API
		auth.Module,   // For API Key validation logic
		apikey.Module, // For API Key domain
		meter.Module,
		usage.Module,
		ratelimit.Module,

		fx.Provide(server.NewEngine),
		fx.Provide(server.NewServer),
		fx.Invoke(func(s *server.Server) {
			s.RegisterAPIRoutes()
		}),
		fx.Invoke(server.RunHTTP),
	)
	app.Run()
}

func RegisterSnowflake() *snowflake.Node {
	node, err := snowflake.NewNode(1)
	if err != nil {
		panic(err)
	}
	return node
}
