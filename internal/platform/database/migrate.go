package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	gooseDB "github.com/pressly/goose/v3/database"
	"github.com/pressly/goose/v3/lock"

	"github.com/sK4rdell/signal-engine/migrations"
)

// MigrationStatus describes one known migration.
type MigrationStatus struct {
	Version int64
	Path    string
	Applied bool
}

// Migrator applies the embedded goose migrations.
type Migrator struct {
	provider *goose.Provider
}

// NewMigrator builds a migrator over pool. goose needs a *sql.DB, so the pool
// is adapted rather than opening a second set of connections. A session-level
// advisory lock serialises concurrent migrators (for example two replicas
// deploying at once).
func NewMigrator(pool *pgxpool.Pool, logger *slog.Logger) (*Migrator, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("database: create migration locker: %w", err)
	}
	provider, err := goose.NewProvider(
		gooseDB.DialectPostgres,
		stdlib.OpenDBFromPool(pool),
		migrations.FS,
		goose.WithSlog(logger),
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("database: create migration provider: %w", err)
	}
	return &Migrator{provider: provider}, nil
}

// Up applies every pending migration and returns the applied versions.
func (m *Migrator) Up(ctx context.Context) ([]int64, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("database: apply migrations: %w", err)
	}
	versions := make([]int64, 0, len(results))
	for _, r := range results {
		versions = append(versions, r.Source.Version)
	}
	return versions, nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) (int64, error) {
	result, err := m.provider.Down(ctx)
	if err != nil {
		return 0, fmt.Errorf("database: roll back migration: %w", err)
	}
	return result.Source.Version, nil
}

// Status lists every known migration and whether it has been applied.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	statuses, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("database: read migration status: %w", err)
	}
	out := make([]MigrationStatus, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, MigrationStatus{
			Version: s.Source.Version,
			Path:    s.Source.Path,
			Applied: s.State == goose.StateApplied,
		})
	}
	return out, nil
}

// Version returns the current schema version.
func (m *Migrator) Version(ctx context.Context) (int64, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("database: read schema version: %w", err)
	}
	return v, nil
}

// MigrateUp applies all pending migrations. It is the one-call form used by
// the test infrastructure.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	m, err := NewMigrator(pool, logger)
	if err != nil {
		return err
	}
	_, err = m.Up(ctx)
	return err
}
