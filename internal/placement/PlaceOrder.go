package placement

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
	"github.com/shubha-9051/predictionMarket/internal/matching"
	"github.com/shubha-9051/predictionMarket/internal/orders"
	"github.com/shubha-9051/predictionMarket/internal/settlement"
)

func PlaceOrder(
	ctx context.Context,
	pool *pgxpool.Pool,
	exchange *matching.Exchange,
	userID int64,
	marketID int64,
	outcome string,
	side string,
	price int64,
	quantity int64,
) ([]matching.Fill, error) {

	maxCost := price * quantity
	if err := ledger.Lock(ctx, pool, userID, maxCost); err != nil {
		return nil, fmt.Errorf("place: lock funds: %w", err)
	}

	orderID, err := orders.CreateOrder(ctx, pool, userID, marketID, outcome, side, price, quantity)
	if err != nil {
		_ = ledger.Release(ctx, pool, userID, maxCost)
		return nil, fmt.Errorf("place: create order: %w", err)
	}

	book := exchange.BookFor(marketID)
	incoming := matching.Order{
		ID:        orderID,
		Outcome:   outcome,
		Side:      side,
		Price:     price,
		Quantity:  quantity,
		Remaining: quantity,
	}
	fills := book.Match(incoming)

	var filledQty int64
	var spent int64
	for _, fill := range fills {
		var err error
		if fill.Type == "mint" {
			err = settlement.SettleMintFill(ctx, pool, fill)
		} else {
			err = settlement.SettleFill(ctx, pool, fill)
		}
		if err != nil {
			return fills, fmt.Errorf("place: settle fill: %w", err)
		}
		filledQty += fill.Quantity
		spent += fill.Price * fill.Quantity
	}

	remaining := quantity - filledQty
	status := "open"
	if remaining == 0 {
		status = "filled"
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fills, err
	}
	defer tx.Rollback(ctx)
	if err := orders.UpdateOrderRemaining(ctx, tx, orderID, remaining, status); err != nil {
		return fills, fmt.Errorf("place: update order: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fills, err
	}

	committed := spent + price*remaining
	overLock := maxCost - committed
	if overLock > 0 {
		if err := ledger.Release(ctx, pool, userID, overLock); err != nil {
			return fills, fmt.Errorf("place: release over-lock: %w", err)
		}
	}

	return fills, nil
}
