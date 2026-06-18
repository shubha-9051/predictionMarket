package settlement

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
)

func TestSettleMint(t *testing.T) {
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
		marketID = int64(7)
		yesPrice = int64(600)
		noPrice  = int64(400)
		qty      = int64(10)
	)
	yesPaid := yesPrice * qty
	noPaid := noPrice * qty

	if err := ledger.Deposit(ctx, pool, yesBuyerID, yesPaid); err != nil {
		t.Fatalf("deposit yes buyer: %v", err)
	}
	if err := ledger.Lock(ctx, pool, yesBuyerID, yesPaid); err != nil {
		t.Fatalf("lock yes buyer: %v", err)
	}
	if err := ledger.Deposit(ctx, pool, noBuyerID, noPaid); err != nil {
		t.Fatalf("deposit no buyer: %v", err)
	}
	if err := ledger.Lock(ctx, pool, noBuyerID, noPaid); err != nil {
		t.Fatalf("lock no buyer: %v", err)
	}

	yesBuyerMoneyBefore := totalMoney(ctx, t, pool, yesBuyerID)
	noBuyerMoneyBefore := totalMoney(ctx, t, pool, noBuyerID)
	vaultBefore := totalMoney(ctx, t, pool, CollateralVaultID)
	yesSharesBefore := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesBefore := shares(ctx, t, pool, noBuyerID, marketID, "NO")

	if err := SettleMint(ctx, pool, yesBuyerID, noBuyerID, marketID, yesPrice, noPrice, qty); err != nil {
		t.Fatalf("SettleMint: %v", err)
	}

	yesBuyerMoneyAfter := totalMoney(ctx, t, pool, yesBuyerID)
	noBuyerMoneyAfter := totalMoney(ctx, t, pool, noBuyerID)
	vaultAfter := totalMoney(ctx, t, pool, CollateralVaultID)
	yesSharesAfter := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesAfter := shares(ctx, t, pool, noBuyerID, marketID, "NO")

	if yesBuyerMoneyBefore-yesBuyerMoneyAfter != yesPaid {
		t.Errorf("yes buyer money: expected -%d, got %d", yesPaid, yesBuyerMoneyBefore-yesBuyerMoneyAfter)
	}
	if noBuyerMoneyBefore-noBuyerMoneyAfter != noPaid {
		t.Errorf("no buyer money: expected -%d, got %d", noPaid, noBuyerMoneyBefore-noBuyerMoneyAfter)
	}

	if vaultAfter-vaultBefore != yesPaid+noPaid {
		t.Errorf("vault: expected +%d, got %d", yesPaid+noPaid, vaultAfter-vaultBefore)
	}

	if yesSharesAfter-yesSharesBefore != qty {
		t.Errorf("yes shares: expected +%d, got %d", qty, yesSharesAfter-yesSharesBefore)
	}
	if noSharesAfter-noSharesBefore != qty {
		t.Errorf("no shares: expected +%d, got %d", qty, noSharesAfter-noSharesBefore)
	}

	if vaultAfter-vaultBefore != PairTotal*qty {
		t.Errorf("vault invariant: expected PairTotal*qty = %d, got %d", PairTotal*qty, vaultAfter-vaultBefore)
	}
}
