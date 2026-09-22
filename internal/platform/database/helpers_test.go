package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pgxPool struct {
	Pool *pgxpool.Pool
}

func (p *pgxPool) count(t *testing.T, ctx context.Context) int {
	t.Helper()
	var n int
	if err := p.Pool.QueryRow(ctx, "SELECT count(*) FROM tx_test").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}
