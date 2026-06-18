package settlement

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
	"github.com/shubha-9051/predictionMarket/internal/matching"
	"github.com/shubha-9051/predictionMarket/internal/orders"
	"github.com/shubha-9051/predictionMarket/internal/positions"
)

func SettleFill(ctx context.Context, pool *pgxpool.Pool, fill matching.Fill) error {
	incoming, err := orders.GetOrder(ctx, pool, fill.IncomingOrderID)
	if err != nil {
		return fmt.Errorf("settle: lookup incoming order %d: %w", fill.IncomingOrderID, err)
	}
	resting, err := orders.GetOrder(ctx, pool, fill.RestingOrderID)
	if err != nil {
		return fmt.Errorf("settle: lookup resting order %d: %w", fill.RestingOrderID, err)
	}

	var buyer, seller orders.Order
	if incoming.Side == "buy" && resting.Side == "sell" {
		buyer, seller = incoming, resting
	} else if incoming.Side == "sell" && resting.Side == "buy" {
		buyer, seller = resting, incoming
	} else {
		return fmt.Errorf("settle: invalid sides incoming=%s resting=%s", incoming.Side, resting.Side)
	}

	if buyer.MarketID != seller.MarketID || buyer.Outcome != seller.Outcome {
		return fmt.Errorf("settle: market/outcome mismatch")
	}

	moneyAmount := fill.Price * fill.Quantity

	restingRemaining := resting.Remaining - fill.Quantity
	restingStatus := "open"
	if restingRemaining == 0 {
		restingStatus = "filled"
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ledger.TransferTx(ctx, tx, buyer.UserID, seller.UserID, moneyAmount); err != nil {
		return fmt.Errorf("settle: money transfer: %w", err)
	}

	if err := positions.TransferSharesTx(ctx, tx, seller.UserID, buyer.UserID,
		buyer.MarketID, buyer.Outcome, fill.Quantity); err != nil {
		return fmt.Errorf("settle: share transfer: %w", err)
	}

	if err := orders.UpdateOrderRemaining(ctx, tx, resting.ID, restingRemaining, restingStatus); err != nil {
		return fmt.Errorf("settle: update resting order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
