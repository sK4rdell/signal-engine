package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sK4rdell/signal-engine/internal/platform/database"
	"github.com/sK4rdell/signal-engine/internal/platform/testutil/pgtest"
)

func setup(t *testing.T) (context.Context, *pgxPool) {
	t.Helper()
	pool := pgtest.NewPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "CREATE TABLE tx_test (id int PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return ctx, &pgxPool{pool}
}

func TestInTx_CommitsOnSuccess(t *testing.T) {
	ctx, db := setup(t)

	err := database.InTx(ctx, db.Pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)")
		return err
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if got := db.count(t, ctx); got != 1 {
		t.Errorf("rows = %d, want 1", got)
	}
}

func TestInTx_RollsBackAndReturnsOriginalErrorOnFailure(t *testing.T) {
	ctx, db := setup(t)
	sentinel := errors.New("boom")

	err := database.InTx(ctx, db.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want sentinel", err)
	}
	if got := db.count(t, ctx); got != 0 {
		t.Errorf("rows = %d, want 0 after rollback", got)
	}
}

func TestInTx_RollsBackWhenCallbackFailsAfterDatabaseError(t *testing.T) {
	ctx, db := setup(t)

	err := database.InTx(ctx, db.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)"); err != nil {
			return err
		}
		// Duplicate key aborts the transaction on the server side.
		_, err := tx.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)")
		return err
	})
	if _, ok := database.UniqueViolation(err); !ok {
		t.Fatalf("expected unique violation, got %v", err)
	}
	if got := db.count(t, ctx); got != 0 {
		t.Errorf("rows = %d, want 0", got)
	}
}

func TestInTx_RespectsContextCancellation(t *testing.T) {
	ctx, db := setup(t)
	cancelCtx, cancel := context.WithCancel(ctx)

	err := database.InTx(cancelCtx, db.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(cancelCtx, "INSERT INTO tx_test (id) VALUES (1)"); err != nil {
			return err
		}
		cancel()
		_, err := tx.Exec(cancelCtx, "INSERT INTO tx_test (id) VALUES (2)")
		return err
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTx error = %v, want context.Canceled", err)
	}
	if got := db.count(t, ctx); got != 0 {
		t.Errorf("rows = %d, want 0 after cancellation", got)
	}
}

func TestInTx_CancelledBeforeBeginFails(t *testing.T) {
	ctx, db := setup(t)
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	called := false
	err := database.InTx(cancelCtx, db.Pool, func(tx pgx.Tx) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if called {
		t.Error("callback must not run when begin fails")
	}
}

func TestInTx_RepanicsAfterRollback(t *testing.T) {
	ctx, db := setup(t)

	func() {
		defer func() {
			if r := recover(); r != "panic in callback" {
				t.Fatalf("recovered %v, want re-raised panic", r)
			}
		}()
		_ = database.InTx(ctx, db.Pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)"); err != nil {
				return err
			}
			panic("panic in callback")
		})
	}()
	if got := db.count(t, ctx); got != 0 {
		t.Errorf("rows = %d, want 0 after panic", got)
	}
}

func TestInTx_ReleasesConnections(t *testing.T) {
	ctx, db := setup(t)

	for i := 0; i < 20; i++ {
		_ = database.InTx(ctx, db.Pool, func(tx pgx.Tx) error {
			if i%2 == 0 {
				return errors.New("fail")
			}
			return nil
		})
	}
	deadline := time.Now().Add(5 * time.Second)
	for db.Pool.Stat().AcquiredConns() != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("acquired connections = %d, want 0", db.Pool.Stat().AcquiredConns())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The pool must still be usable.
	if err := db.Pool.Ping(ctx); err != nil {
		t.Fatalf("ping after transactions: %v", err)
	}
}

func TestUniqueViolation_ReportsConstraintName(t *testing.T) {
	ctx, db := setup(t)
	if _, err := db.Pool.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	_, err := db.Pool.Exec(ctx, "INSERT INTO tx_test (id) VALUES (1)")
	name, ok := database.UniqueViolation(err)
	if !ok || name != "tx_test_pkey" {
		t.Fatalf("UniqueViolation = %q,%v", name, ok)
	}
	if !database.IsUniqueViolation(err, "tx_test_pkey") {
		t.Error("IsUniqueViolation should match")
	}
	if database.IsUniqueViolation(errors.New("other"), "tx_test_pkey") {
		t.Error("plain errors are not violations")
	}
}
