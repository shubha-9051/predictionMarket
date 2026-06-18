package orders

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateOrder(ctx context.Context, pool *pgxpool.Pool, userID int64, marketID int64, outcome string, side string, price int64, quantity int64) (int64, error) {

	if quantity <= 0 {
		return 0, errors.New("quantity must be positive")
	}
	if price < 1 || price > 999 {
		return 0, errors.New("price must be between 1 and 999")
	}
	if side != "buy" && side != "sell" {
		return 0, errors.New("side must be 'buy' or 'sell'")
	}
	if outcome != "YES" && outcome != "NO" {
		return 0, errors.New("outcome must be 'YES' or 'NO'")
	}

	var id int64
	err := pool.QueryRow(ctx,
		`INSERT INTO orders (user_id, market_id, outcome, side, price, quantity, remaining, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $6, 'open')
		 RETURNING id`,
		userID, marketID, outcome, side, price, quantity).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}
