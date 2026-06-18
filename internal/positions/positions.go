package positions

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreditSharesTx(ctx context.Context, tx pgx.Tx, userID int64, marketID int64, outcome string, quantity int64) error {
	if quantity < 1 {
		return errors.New("quantity is less than 1")
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO positions (user_id, market_id, outcome, quantity)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (user_id, market_id, outcome)
					DO UPDATE SET quantity = positions.quantity + $4`,
		userID, marketID, outcome, quantity)

	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO share_ledger (user_id, market_id, outcome, amount, reason)
			 VALUES ($1, $2, $3, $4, $5)`,
		userID, marketID, outcome, quantity, "Credit")

	if err != nil {
		return err
	}

	return nil
}

func CreditShares(ctx context.Context, pool *pgxpool.Pool, userID int64, marketID int64, outcome string, quantity int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := CreditSharesTx(ctx, tx, userID, marketID, outcome, quantity); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func DebitSharesTx(ctx context.Context, tx pgx.Tx, userID int64, marketID int64, outcome string, quantity int64) error {
	if quantity < 1 {
		return errors.New("quantity is less than 1")
	}

	result, err := tx.Exec(ctx,
		`UPDATE positions SET quantity = quantity - $1
					WHERE user_id = $2
					AND market_id = $3
					AND outcome = $4
					AND quantity >= $1`,
		quantity, userID, marketID, outcome)

	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("position not found or insufficient shares")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO share_ledger (user_id, market_id, outcome, amount, reason)
			VALUES ($1, $2, $3, $4, $5)`,
		userID, marketID, outcome, -quantity, "Debit")

	if err != nil {
		return err
	}

	return nil
}

func DebitShares(ctx context.Context, pool *pgxpool.Pool, userID int64, marketID int64, outcome string, quantity int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := DebitSharesTx(ctx, tx, userID, marketID, outcome, quantity); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func TransferSharesTx(ctx context.Context, tx pgx.Tx, fromUserID int64, toUserID int64, marketID int64, outcome string, quantity int64) error {
	if quantity < 1 {
		return errors.New("quantity must be positive")
	}
	if fromUserID == toUserID {
		return errors.New("cannot transfer shares to self")
	}

	result, err := tx.Exec(ctx,
		`UPDATE positions SET quantity = quantity - $1
		 WHERE user_id = $2 AND market_id = $3 AND outcome = $4 AND quantity >= $1`,
		quantity, fromUserID, marketID, outcome)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("seller position not found or insufficient shares")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO positions (user_id, market_id, outcome, quantity)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (user_id, market_id, outcome)
		 DO UPDATE SET quantity = positions.quantity + $4`,
		toUserID, marketID, outcome, quantity)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO share_ledger (user_id, market_id, outcome, amount, reason)
		 VALUES ($1, $2, $3, $4, $5)`,
		fromUserID, marketID, outcome, -quantity, "transfer_out")
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO share_ledger (user_id, market_id, outcome, amount, reason)
		 VALUES ($1, $2, $3, $4, $5)`,
		toUserID, marketID, outcome, quantity, "transfer_in")
	if err != nil {
		return err
	}

	return nil
}

func TransferShares(ctx context.Context, pool *pgxpool.Pool, fromUserID int64, toUserID int64, marketID int64, outcome string, quantity int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := TransferSharesTx(ctx, tx, fromUserID, toUserID, marketID, outcome, quantity); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
