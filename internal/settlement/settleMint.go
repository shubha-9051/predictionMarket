package settlement

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
	"github.com/shubha-9051/predictionMarket/internal/positions"
)

const CollateralVaultID = 73

const PairTotal = 1000

func SettleMint(
	ctx context.Context,
	pool *pgxpool.Pool,
	yesBuyerID int64,
	noBuyerID int64,
	marketID int64,
	yesPrice int64,
	noPrice int64,
	qty int64,
) error {
	if qty < 1 {
		return fmt.Errorf("mint: quantity must be positive")
	}
	if yesPrice+noPrice < PairTotal {
		return fmt.Errorf("mint: prices %d + %d do not cover pair total %d", yesPrice, noPrice, PairTotal)
	}

	yesPaid := yesPrice * qty
	noPaid := noPrice * qty

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ledger.TransferTx(ctx, tx, yesBuyerID, CollateralVaultID, yesPaid); err != nil {
		return fmt.Errorf("mint: collect YES collateral: %w", err)
	}
	if err := ledger.TransferTx(ctx, tx, noBuyerID, CollateralVaultID, noPaid); err != nil {
		return fmt.Errorf("mint: collect NO collateral: %w", err)
	}

	if err := positions.CreditSharesTx(ctx, tx, yesBuyerID, marketID, "YES", qty); err != nil {
		return fmt.Errorf("mint: create YES shares: %w", err)
	}
	if err := positions.CreditSharesTx(ctx, tx, noBuyerID, marketID, "NO", qty); err != nil {
		return fmt.Errorf("mint: create NO shares: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
