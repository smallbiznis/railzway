package main

import (
	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/migration"
	"github.com/smallbiznis/railzway/internal/observability"
	"github.com/smallbiznis/railzway/internal/scheduler"
	"github.com/smallbiznis/railzway/internal/server"
	"github.com/smallbiznis/railzway/pkg/db"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		// Core Infrastructure
		// config.Module,
		observability.Module,
		fx.Provide(RegisterSnowflake),
		db.Module,
		clock.Module,
		server.Module,

		// Functional Domains
		scheduler.Module,
		migration.Module,

		// All other domain modules usually imported by specific apps
		// We can mostly rely on server.Module importing them transitively or explicitly here
		// but server.Module already imports MOST of them.
		
		// server.Module now invokes RegisterRoutes automatically.


		// RunHTTP is invoked by server.Module or explicitly?
		// server.Module has fx.Invoke(RunHTTP).
		// We can leave it or be explicit.
		// To be safe, let's Suppress server.Module's autodrive if needed, or just let it run.
		// But server.Module defines `fx.Invoke(RunHTTP)` at line 125.
		// So it will run automatically.
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
