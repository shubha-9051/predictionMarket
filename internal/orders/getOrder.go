package orders

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Order struct {
	ID        int64
	UserID    int64
	MarketID  int64
	Outcome   string
	Side      string
	Price     int64
	Quantity  int64
	Remaining int64
	Status    string
}

func GetOrder(ctx context.Context, pool *pgxpool.Pool, id int64) (Order, error) {
	var o Order
	err := pool.QueryRow(ctx,
		`SELECT id, user_id, market_id, outcome, side, price, quantity, remaining, status
		 FROM orders WHERE id = $1`,
		id).Scan(&o.ID, &o.UserID, &o.MarketID, &o.Outcome, &o.Side,
		&o.Price, &o.Quantity, &o.Remaining, &o.Status)
	if err != nil {
		return Order{}, err
	}
	return o, nil
}
