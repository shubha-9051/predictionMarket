package settlement

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
	"github.com/shubha-9051/predictionMarket/internal/positions"
)

func ResolveMarket(ctx context.Context, pool *pgxpool.Pool, marketID int64, winningOutcome string) error {
	if winningOutcome != "YES" && winningOutcome != "NO" {
		return fmt.Errorf("resolve: winning outcome must be YES or NO, got %q", winningOutcome)
	}

	rows, err := pool.Query(ctx,
		`SELECT user_id, outcome, quantity FROM positions
		 WHERE market_id = $1 AND quantity > 0`,
		marketID)
	if err != nil {
		return fmt.Errorf("resolve: query positions: %w", err)
	}

	type pos struct {
		userID   int64
		outcome  string
		quantity int64
	}
	var allPositions []pos
	for rows.Next() {
		var p pos
		if err := rows.Scan(&p.userID, &p.outcome, &p.quantity); err != nil {
			rows.Close()
			return fmt.Errorf("resolve: scan position: %w", err)
		}
		allPositions = append(allPositions, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("resolve: row iteration: %w", err)
	}

	for _, p := range allPositions {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("resolve: begin tx: %w", err)
		}

		if p.outcome == winningOutcome {
			payout := PairTotal * p.quantity
			if err := ledger.PayoutTx(ctx, tx, CollateralVaultID, p.userID, payout); err != nil {
				tx.Rollback(ctx)
				return fmt.Errorf("resolve: payout to user %d: %w", p.userID, err)
			}
		}

		if err := positions.DebitSharesTx(ctx, tx, p.userID, marketID, p.outcome, p.quantity); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("resolve: remove shares of user %d: %w", p.userID, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("resolve: commit for user %d: %w", p.userID, err)
		}
	}

	return nil
}
