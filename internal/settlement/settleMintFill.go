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

func SettleMintFill(ctx context.Context, pool *pgxpool.Pool, fill matching.Fill) error {
	incoming, err := orders.GetOrder(ctx, pool, fill.IncomingOrderID)
	if err != nil {
		return fmt.Errorf("mint: lookup incoming order %d: %w", fill.IncomingOrderID, err)
	}
	resting, err := orders.GetOrder(ctx, pool, fill.RestingOrderID)
	if err != nil {
		return fmt.Errorf("mint: lookup resting order %d: %w", fill.RestingOrderID, err)
	}

	var yesBuyer, noBuyer orders.Order
	if incoming.Outcome == "YES" && resting.Outcome == "NO" {
		yesBuyer, noBuyer = incoming, resting
	} else if incoming.Outcome == "NO" && resting.Outcome == "YES" {
		yesBuyer, noBuyer = resting, incoming
	} else {
		return fmt.Errorf("mint: expected opposite outcomes, got incoming=%s resting=%s",
			incoming.Outcome, resting.Outcome)
	}

	if yesBuyer.Side != "buy" || noBuyer.Side != "buy" {
		return fmt.Errorf("mint: both orders must be buys, got yes=%s no=%s",
			yesBuyer.Side, noBuyer.Side)
	}

	if yesBuyer.MarketID != noBuyer.MarketID {
		return fmt.Errorf("mint: market mismatch")
	}

	yesPrice := yesBuyer.Price
	noPrice := noBuyer.Price
	if yesPrice+noPrice < PairTotal {
		return fmt.Errorf("mint: prices %d + %d < pair total %d", yesPrice, noPrice, PairTotal)
	}

	qty := fill.Quantity
	yesPaid := yesPrice * qty
	noPaid := noPrice * qty

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ledger.TransferTx(ctx, tx, yesBuyer.UserID, CollateralVaultID, yesPaid); err != nil {
		return fmt.Errorf("mint: collect YES collateral: %w", err)
	}
	if err := ledger.TransferTx(ctx, tx, noBuyer.UserID, CollateralVaultID, noPaid); err != nil {
		return fmt.Errorf("mint: collect NO collateral: %w", err)
	}
	if err := positions.CreditSharesTx(ctx, tx, yesBuyer.UserID, yesBuyer.MarketID, "YES", qty); err != nil {
		return fmt.Errorf("mint: create YES shares: %w", err)
	}
	if err := positions.CreditSharesTx(ctx, tx, noBuyer.UserID, noBuyer.MarketID, "NO", qty); err != nil {
		return fmt.Errorf("mint: create NO shares: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
