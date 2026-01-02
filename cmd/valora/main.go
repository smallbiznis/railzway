package main

import (
	"context"

	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/internal/migration"
	"github.com/smallbiznis/valora/internal/observability"
	"github.com/smallbiznis/valora/internal/scheduler"
	"github.com/smallbiznis/valora/internal/seed"
	"github.com/smallbiznis/valora/internal/server"
	"github.com/smallbiznis/valora/pkg/db"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var version = "dev"

func main() {
	app := fx.New(
		observability.Module,
		fx.Provide(func() *snowflake.Node {
			node, err := snowflake.NewNode(1)
			if err != nil {
				panic(err)
			}
			return node
		}),
		db.Module,
		fx.Provide(func(cfg config.Config) scheduler.Config {
			schedulerCfg := scheduler.DefaultConfig()
			if !cfg.IsCloud() {
				schedulerCfg.FinalizeInvoices = true
			}
			return schedulerCfg
		}),
		fx.Provide(scheduler.New),
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
		fx.Invoke(func(lc fx.Lifecycle, cfg config.Config, sched *scheduler.Scheduler) {
			if cfg.IsCloud() {
				return
			}
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go sched.RunForever(ctx)
					return nil
				},
			})
		}),
	)
	app.Run()
}
