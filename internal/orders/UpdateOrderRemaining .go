package orders

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func UpdateOrderRemaining(ctx context.Context, tx pgx.Tx, orderID int64, remaining int64, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET remaining = $1, status = $2 WHERE id = $3`,
		remaining, status, orderID)
	if err != nil {
		return err
	}
	return nil
}
