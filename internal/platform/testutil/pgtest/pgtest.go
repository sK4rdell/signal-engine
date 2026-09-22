// Package pgtest provides isolated PostgreSQL databases for tests without
// depending on any feature package, so repository tests inside feature
// packages can use it.
package pgtest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/platform/config"
	"betemplate/internal/platform/database"
	"betemplate/internal/platform/logging"
	"betemplate/migrations"
)

// Every test process cooperates through this advisory lock when creating
// databases, so parallel packages never race on the template.
const templateLockID = 0x6265_7465_6d70 // "betemp"

// Isolation strategy: one template database per migration set, migrated
// once, then a throwaway copy per test created with CREATE DATABASE ...
// TEMPLATE. Copying is far cheaper than migrating, and every test still gets
// a fully isolated database that survives commits, workers and concurrent
// connections.
var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

// NewPool returns a connection pool to a freshly created, fully migrated
// database that is dropped when the test finishes.
//
// The test is skipped when TEST_DATABASE_URL is unset.
func NewPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dsn := DatabaseURL(t)

	adminURL := dsn
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	templateOnce.Do(func() {
		templateName, templateErr = ensureTemplate(ctx, adminURL)
	})
	if templateErr != nil {
		t.Fatalf("pgtest: prepare template database: %v", templateErr)
	}

	name := "t_" + randomHex(8)
	if err := createDatabase(ctx, adminURL, name, templateName); err != nil {
		t.Fatalf("pgtest: create test database: %v", err)
	}

	pool, err := database.NewPool(ctx, config.DatabaseConfig{
		URL:               withDatabase(adminURL, name),
		MaxConns:          8,
		MaxConnLifetime:   time.Hour,
		MaxConnIdleTime:   time.Minute,
		HealthCheckPeriod: time.Minute,
	})
	if err != nil {
		_ = dropDatabase(context.Background(), adminURL, name)
		t.Fatalf("pgtest: connect to test database: %v", err)
	}

	t.Cleanup(func() {
		ClosePool(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := dropDatabase(ctx, adminURL, name); err != nil {
			t.Errorf("pgtest: drop test database %s: %v", name, err)
		}
	})
	return pool
}

// DatabaseURL returns TEST_DATABASE_URL or skips the test.
func DatabaseURL(t testing.TB) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL test")
	}
	return dsn
}

// ClosePool closes pool and fails the test instead of hanging when a
// connection or transaction leaked.
func ClosePool(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	closed := make(chan struct{})
	go func() {
		pool.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		stat := pool.Stat()
		t.Errorf("pgtest: pool did not close within 5s: acquired=%d total=%d idle=%d (leaked connection or transaction?)",
			stat.AcquiredConns(), stat.TotalConns(), stat.IdleConns())
	}
}

func ensureTemplate(ctx context.Context, adminURL string) (string, error) {
	hash, err := migrationsHash()
	if err != nil {
		return "", err
	}
	name := "tmpl_" + hash

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", fmt.Errorf("connect to TEST_DATABASE_URL: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", templateLockID); err != nil {
		return "", fmt.Errorf("acquire template lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", templateLockID) }()

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		return "", fmt.Errorf("check template: %w", err)
	}
	if exists {
		return name, nil
	}

	if err := dropStaleTemplates(ctx, conn, name); err != nil {
		return "", err
	}

	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		return "", fmt.Errorf("create template database: %w", err)
	}

	pool, err := pgxpool.New(ctx, withDatabase(adminURL, name))
	if err != nil {
		return "", fmt.Errorf("connect to template database: %w", err)
	}
	migrateErr := database.MigrateUp(ctx, pool, logging.Discard())
	pool.Close()
	if migrateErr != nil {
		_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		return "", fmt.Errorf("migrate template database: %w", migrateErr)
	}
	return name, nil
}

// staleTemplateSQL matches only databases with the exact generated template
// name format (tmpl_ plus 12 hex characters), never other databases that a
// LIKE 'tmpl_%' pattern would also match, because they are force-dropped.
const staleTemplateSQL = `SELECT datname FROM pg_database WHERE datname ~ '^tmpl_[0-9a-f]{12}$' AND datname <> $1`

// dropStaleTemplates removes templates built from older migration sets.
func dropStaleTemplates(ctx context.Context, conn *pgx.Conn, keep string) error {
	rows, err := conn.Query(ctx, staleTemplateSQL, keep)
	if err != nil {
		return fmt.Errorf("list stale templates: %w", err)
	}
	stale, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("read stale templates: %w", err)
	}
	for _, s := range stale {
		_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{s}.Sanitize()+" WITH (FORCE)")
	}
	return nil
}

func createDatabase(ctx context.Context, adminURL, name, template string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// Serialised: PostgreSQL refuses to copy a template that another
	// session is copying at the same moment.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", templateLockID); err != nil {
		return fmt.Errorf("acquire template lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", templateLockID) }()

	sql := fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", pgx.Identifier{name}.Sanitize(), pgx.Identifier{template}.Sanitize())
	if _, err := conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return nil
}

func dropDatabase(ctx context.Context, adminURL, name string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	return err
}

// migrationsHash identifies the embedded migration set so a template is
// rebuilt whenever a migration changes.
func migrationsHash() (string, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		content, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return "", err
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(content)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12], nil
}

func withDatabase(dsn, name string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	u.Path = "/" + name
	return u.String()
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return strings.ToLower(hex.EncodeToString(b))
}
