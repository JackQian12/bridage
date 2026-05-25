package postgres

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates and validates a pgxpool connection pool.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

// RunMigrations applies all pending up migrations from the given fs.FS.
// The FS must contain *.sql files at its root (or a subdirectory passed as subFS).
func RunMigrations(databaseURL string, migrationsFS fs.FS) error {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
	}
	// golang-migrate expects postgres:// URL; pgx5:// driver also accepts postgres://
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+databaseURL[len("postgres://"):])
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
