package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/logging"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil/pgtest"
)

// TestMigrator_UpDownUpFromEmptyDatabase applies every migration to an empty
// database, rolls each one back and applies them again. This proves the
// schema can be rebuilt from the repository alone and that Down sections
// are correct.
func TestMigrator_UpDownUpFromEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	adminURL := pgtest.DatabaseURL(t)

	// A raw empty database rather than the migrated template.
	name := "mig_" + t.Name()[len("TestMigrator_"):]
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)")
	})

	cfg, err := pgxpool.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	m, err := database.NewMigrator(pool, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}

	applied, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("Up: %v", err)
	}
	if len(applied) == 0 {
		t.Fatal("expected migrations to be applied")
	}

	statuses, err := m.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for _, s := range statuses {
		if !s.Applied {
			t.Errorf("migration %d not applied", s.Version)
		}
	}

	for range applied {
		if _, err := m.Down(ctx); err != nil {
			t.Fatalf("Down: %v", err)
		}
	}
	version, err := m.Version(ctx)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if version != 0 {
		t.Errorf("version after full rollback = %d, want 0", version)
	}

	var tables int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename <> 'goose_db_version'").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Errorf("%d tables remain after rollback", tables)
	}

	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("second Up: %v", err)
	}
}
