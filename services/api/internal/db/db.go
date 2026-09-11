// Package db manages the PostgreSQL connection pool and runs migrations.
package db

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Pool wraps pgxpool.Pool with health check helpers.
type Pool struct {
	*pgxpool.Pool
}

// Connect opens a connection pool and verifies connectivity.
// migrationPath is the filesystem path to the migrations directory.
// Pass an empty string to skip running migrations (e.g., in tests that manage their own schema).
func Connect(ctx context.Context, databaseURL string, migrationPath string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	// Connection pool tuning
	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}
	log.Info().Msg("database connection pool established")

	p := &Pool{Pool: pool}

	if migrationPath != "" {
		if err := p.RunMigrations(migrationPath); err != nil {
			pool.Close()
			return nil, fmt.Errorf("db: migrations failed: %w", err)
		}
	}

	return p, nil
}

// RunMigrations applies all pending UP migrations from the given path.
// This is idempotent — it is safe to call on every startup.
func (p *Pool) RunMigrations(migrationsPath string) error {
	log.Info().Str("path", migrationsPath).Msg("running database migrations")

	m, err := migrate.New(
		migrateFileURL(migrationsPath),
		p.Config().ConnString(),
	)
	if err != nil {
		return fmt.Errorf("db: create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("db: apply migrations: %w", err)
	}

	v, dirty, _ := m.Version()
	log.Info().Uint("migration_version", v).Bool("dirty", dirty).Msg("migrations complete")
	return nil
}

// Ping checks database connectivity. Used in the /healthz endpoint.
func (p *Pool) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}

// migrateFileURL converts an OS path to a file:// URL compatible with golang-migrate.
// On Windows, backslashes are replaced with forward slashes and an extra slash
// is added after "file://" so that drive letters are correctly interpreted.
// e.g. C:\Users\... → file:///C:/Users/...
func migrateFileURL(path string) string {
	// Convert to absolute to make sure we have no relative components
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// Normalise to forward slashes (no-op on Linux/macOS)
	clean := strings.ReplaceAll(abs, "\\", "/")
	// On Windows, abs starts with a drive letter like "C:/..."
	// We need file:///C:/... (three slashes)
	if len(clean) >= 2 && clean[1] == ':' {
		return "file:///" + clean
	}
	// Unix absolute path: /path/to/migrations → file:///path/to/migrations
	if strings.HasPrefix(clean, "/") {
		return "file://" + clean
	}
	return "file://" + clean
}
