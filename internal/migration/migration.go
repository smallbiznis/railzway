package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// This migration package ensures Railzway is fully usable
// out of the box for local and self-hosted environments.
// All core billing tables are created automatically on startup.
func RunMigrations(db *sql.DB) error {
	if db == nil {
		return errors.New("migration database handle is required")
	}

	sub, err := fs.Sub(embeddedMigrations, migrationsDir)
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}

	source, err := iofs.New(sub, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	upErr := migrator.Up()
	if upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", upErr)
	}
	// Do not call migrator.Close here because it would close the shared *sql.DB.

	return nil
}
