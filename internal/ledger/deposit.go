package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Deposit(ctx context.Context, pool *pgxpool.Pool, userID int64, amount int64) error {
	// Guard: reject nonsensical input before touching the database.
	if amount <= 0 {
		return errors.New("amount must be positive")
	}

	// Begin the transaction. Check the error BEFORE deferring rollback,
	// because if Begin failed, tx is nil and rolling back would panic.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Safety net: undo everything unless we explicitly commit.
	// A rollback after a successful commit is harmless (pgx ignores it).
	defer tx.Rollback(ctx)

	// Write 1: increase the balance, computed relative to current value.
	result, err := tx.Exec(ctx,
		"UPDATE users SET available = available + $1 WHERE id = $2",
		amount, userID)
	if err != nil {
		return err
	}
	// Catch the silent no-op: if no row matched, the user doesn't exist.
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", userID)
	}

	// Write 2: record the deposit in the append-only ledger.
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (user_id, amount, reason) VALUES ($1, $2, $3)",
		userID, amount, "deposit")
	if err != nil {
		return err
	}

	// Commit makes both writes permanent — atomically.
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}
