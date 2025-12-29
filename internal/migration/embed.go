package migration

import "embed"

const migrationsDir = "migrations"

//go:embed migrations/*.up.sql
var embeddedMigrations embed.FS
