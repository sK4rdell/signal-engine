package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InTx runs fn inside a transaction.
//
// The transaction is committed only when fn returns nil. On any other path
// (fn error, context cancellation, panic) it is rolled back and the
// connection is returned to the pool. A panic in fn is re-raised after the
// rollback. The error returned by fn is returned unchanged so callers can
// inspect it with errors.Is and errors.As.
func InTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("database: begin transaction: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		// Rollback after a failed commit or a closed connection returns
		// pgx.ErrTxClosed; the primary error is what matters.
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("database: rollback: %w", rbErr))
		}
		committed = true // nothing left to roll back
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("database: commit transaction: %w", err)
	}
	committed = true
	return nil
}
