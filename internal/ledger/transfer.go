package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Transfer(ctx context.Context, pool *pgxpool.Pool, fromUserID int64, toUserID int64, amount int64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result1, err := tx.Exec(ctx,
		`UPDATE users SET locked = locked - $1 
		 WHERE id = $2 AND locked >= $1`,
		amount, fromUserID)

	if err != nil {
		return err
	}
	if result1.RowsAffected() == 0 {
		return fmt.Errorf("transfer failed: user %d not found or insufficient locked funds", fromUserID)
	}

	result2, err := tx.Exec(ctx,
		`UPDATE users SET available = available + $1 
		 WHERE id = $2 `,
		amount, toUserID)

	if err != nil {
		return err
	}
	if result2.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", toUserID)
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (user_id, amount, reason) VALUES ($1, $2, $3)",
		fromUserID, -amount, "transfer_out")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (user_id, amount, reason) VALUES ($1, $2, $3)",
		toUserID, amount, "transfer_in")
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}
