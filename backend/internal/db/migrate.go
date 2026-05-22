package db

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Direction is the direction of a migration run.
type Direction string

const (
	Up   Direction = "up"
	Down Direction = "down"
)

// Migrate applies migrations from migrationsDir to databaseURL.
// migrationsDir is a filesystem path. databaseURL is a pgx-compatible URL.
//
// Returns nil on success or when there are no pending migrations.
func Migrate(databaseURL, migrationsDir string, dir Direction) error {
	source := fmt.Sprintf("file://%s", migrationsDir)
	m, err := migrate.New(source, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate: open: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	switch dir {
	case Up:
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate up: %w", err)
		}
	case Down:
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate down: %w", err)
		}
	default:
		return fmt.Errorf("migrate: unknown direction %q", dir)
	}
	return nil
}
