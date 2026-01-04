package main

import (
	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/migration"
	"github.com/smallbiznis/valora/internal/observability"
	"github.com/smallbiznis/valora/internal/scheduler"
	"github.com/smallbiznis/valora/internal/server"
	"github.com/smallbiznis/valora/pkg/db"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		observability.Module,
		fx.Provide(RegisterSnowflake),
		db.Module,
		scheduler.Module,
		migration.Module,
		server.Module,
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
