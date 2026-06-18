package ledger

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func PayoutTx(ctx context.Context, tx pgx.Tx, fromUserID int64, toUserID int64, amount int64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	if fromUserID == toUserID {
		return errors.New("cannot pay to self")
	}

	result, err := tx.Exec(ctx,
		`UPDATE users SET available = available - $1
		 WHERE id = $2 AND available >= $1`,
		amount, fromUserID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("payout failed: user %d not found or insufficient available", fromUserID)
	}

	result, err = tx.Exec(ctx,
		`UPDATE users SET available = available + $1 WHERE id = $2`,
		amount, toUserID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("payout failed: receiver %d not found", toUserID)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_entries (user_id, amount, reason) VALUES ($1, $2, $3)`,
		fromUserID, -amount, "payout_out")
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO ledger_entries (user_id, amount, reason) VALUES ($1, $2, $3)`,
		toUserID, amount, "payout_in")
	if err != nil {
		return err
	}

	return nil
}
