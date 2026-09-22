// Package testutil provides the integration-test harness: a fully wired
// application on an isolated PostgreSQL database, factories, an
// authenticated HTTP client and response helpers. The database lifecycle
// itself lives in the pgtest subpackage.
package testutil

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"betemplate/internal/platform/testutil/pgtest"
)

// NewPool returns a pool on a fresh migrated database. See pgtest.NewPool.
func NewPool(t testing.TB) *pgxpool.Pool { return pgtest.NewPool(t) }
