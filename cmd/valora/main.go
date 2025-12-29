package main

import (
	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/internal/logger"
	"github.com/smallbiznis/valora/internal/migration"
	"github.com/smallbiznis/valora/internal/seed"
	"github.com/smallbiznis/valora/internal/server"
	"github.com/smallbiznis/valora/pkg/db"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var version = "dev"

func main() {
	app := fx.New(
		logger.Module,
		fx.Provide(func() *snowflake.Node {
			node, err := snowflake.NewNode(1)
			if err != nil {
				panic(err)
			}
			return node
		}),
		db.Module,
		fx.Invoke(func(conn *gorm.DB, cfg config.Config) error {
			sqlDB, err := conn.DB()
			if err != nil {
				return err
			}
			if err := migration.RunMigrations(sqlDB); err != nil {
				return err
			}
			if err := seed.EnsureMainOrg(conn); err != nil {
				return err
			}
			if !cfg.IsCloud() && cfg.Bootstrap.EnsureDefaultOrgAndUser {
				return seed.EnsureMainOrgAndAdmin(conn)
			}
			return nil
		}),
		server.Module,
	)
	app.Run()
}
