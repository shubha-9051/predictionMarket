package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Release(ctx context.Context, pool *pgxpool.Pool, userID int64, amount int64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx,
		`UPDATE users SET locked = locked - $1, 
		 available = available + $1 
		 WHERE id = $2 AND locked >= $1`,
		amount, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user %d not found", userID)
	}

	_, err = tx.Exec(ctx,
		"INSERT INTO ledger_entries (user_id, amount, reason) VALUES ($1, $2, $3)",
		userID, amount, "release")
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}
