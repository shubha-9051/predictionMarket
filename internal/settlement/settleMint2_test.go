package settlement

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/shubha-9051/predictionMarket/internal/matching"
)

func TestResolveMarket(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	yesBuyerID := createUser(ctx, t, pool)
	noBuyerID := createUser(ctx, t, pool)

	const (
		marketID = int64(30)
		yesPrice = int64(600)
		noPrice  = int64(400)
		qty      = int64(20)
	)

	mustDepositAndLock(ctx, t, pool, yesBuyerID, yesPrice*qty)
	mustDepositAndLock(ctx, t, pool, noBuyerID, noPrice*qty)
	if err := SettleMint(ctx, pool, yesBuyerID, noBuyerID, marketID, yesPrice, noPrice, qty); err != nil {
		t.Fatalf("seed mint: %v", err)
	}

	yesMoneyBefore := totalMoney(ctx, t, pool, yesBuyerID)
	noMoneyBefore := totalMoney(ctx, t, pool, noBuyerID)
	vaultBefore := totalMoney(ctx, t, pool, CollateralVaultID)

	if err := ResolveMarket(ctx, pool, marketID, "YES"); err != nil {
		t.Fatalf("ResolveMarket: %v", err)
	}

	yesMoneyAfter := totalMoney(ctx, t, pool, yesBuyerID)
	noMoneyAfter := totalMoney(ctx, t, pool, noBuyerID)
	vaultAfter := totalMoney(ctx, t, pool, CollateralVaultID)

	expectedPayout := matching.PairTotal * qty

	if yesMoneyAfter-yesMoneyBefore != expectedPayout {
		t.Errorf("yes winner payout: expected +%d, got %d", expectedPayout, yesMoneyAfter-yesMoneyBefore)
	}

	if noMoneyAfter-noMoneyBefore != 0 {
		t.Errorf("no loser: expected +0, got %d", noMoneyAfter-noMoneyBefore)
	}

	yesSharesAfter := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesAfter := shares(ctx, t, pool, noBuyerID, marketID, "NO")
	if yesSharesAfter != 0 {
		t.Errorf("yes shares after resolution: expected 0, got %d", yesSharesAfter)
	}
	if noSharesAfter != 0 {
		t.Errorf("no shares after resolution: expected 0, got %d", noSharesAfter)
	}

	if vaultBefore-vaultAfter != expectedPayout {
		t.Errorf("vault drain: expected -%d, got %d", expectedPayout, vaultBefore-vaultAfter)
	}
}
