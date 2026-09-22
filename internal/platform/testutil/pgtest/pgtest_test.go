package pgtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestDropStaleTemplates_OnlyDropsGeneratedNames creates databases that a
// naive LIKE 'tmpl_%' pattern would match and checks that only the exact
// generated format is dropped.
func TestDropStaleTemplates_OnlyDropsGeneratedNames(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// keep must be the real current template: other test processes may be
	// copying it right now, and dropStaleTemplates drops every other
	// template with the generated name format.
	hash, err := migrationsHash()
	if err != nil {
		t.Fatal(err)
	}
	keep := "tmpl_" + hash
	stale := "tmpl_" + randomHex(6)            // an old template: dropped
	unrelated := "tmplXbackup_" + randomHex(4) // LIKE 'tmpl_%' would match it
	notHex := "tmpl_keep_" + randomHex(4)      // right prefix, wrong format

	// Serialise with other test processes, which also create databases.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", templateLockID); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", templateLockID) }()

	var created []string
	t.Cleanup(func() {
		for _, name := range created {
			_, _ = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		}
	})
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{keep}.Sanitize()); err != nil {
		// Already built by ensureTemplate in this or another process: fine,
		// and then it is not ours to drop.
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42P04" {
			t.Fatal(err)
		}
	} else {
		created = append(created, keep)
	}
	for _, name := range []string{stale, unrelated, notHex} {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatal(err)
		}
		created = append(created, name)
	}

	if err := dropStaleTemplates(ctx, conn, keep); err != nil {
		t.Fatal(err)
	}

	exists := func(name string) bool {
		var ok bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&ok); err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if exists(stale) {
		t.Error("stale template was not dropped")
	}
	for _, name := range []string{keep, unrelated, notHex} {
		if !exists(name) {
			t.Errorf("%s was dropped", name)
		}
	}
}
