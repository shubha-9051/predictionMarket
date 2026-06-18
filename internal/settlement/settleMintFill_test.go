package settlement

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
	"github.com/shubha-9051/predictionMarket/internal/matching"
	"github.com/shubha-9051/predictionMarket/internal/orders"
)

func TestSettleMintFill(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	t.Run("YES incoming", func(t *testing.T) {
		runMintFillCase(ctx, t, pool, true)
	})
	t.Run("NO incoming", func(t *testing.T) {
		runMintFillCase(ctx, t, pool, false)
	})
}

func runMintFillCase(ctx context.Context, t *testing.T, pool *pgxpool.Pool, yesIsIncoming bool) {
	yesBuyerID := createUser(ctx, t, pool)
	noBuyerID := createUser(ctx, t, pool)

	const (
		marketID = int64(9)
		yesPrice = int64(600)
		noPrice  = int64(400)
		qty      = int64(10)
	)
	yesPaid := yesPrice * qty
	noPaid := noPrice * qty

	mustDepositAndLock(ctx, t, pool, yesBuyerID, yesPaid)
	mustDepositAndLock(ctx, t, pool, noBuyerID, noPaid)

	yesOrderID, err := orders.CreateOrder(ctx, pool, yesBuyerID, marketID, "YES", "buy", yesPrice, qty)
	if err != nil {
		t.Fatalf("create yes order: %v", err)
	}
	noOrderID, err := orders.CreateOrder(ctx, pool, noBuyerID, marketID, "NO", "buy", noPrice, qty)
	if err != nil {
		t.Fatalf("create no order: %v", err)
	}

	var fill matching.Fill
	if yesIsIncoming {
		fill = matching.Fill{IncomingOrderID: yesOrderID, RestingOrderID: noOrderID, Quantity: qty, Type: "mint"}
	} else {
		fill = matching.Fill{IncomingOrderID: noOrderID, RestingOrderID: yesOrderID, Quantity: qty, Type: "mint"}
	}

	yesMoneyBefore := totalMoney(ctx, t, pool, yesBuyerID)
	noMoneyBefore := totalMoney(ctx, t, pool, noBuyerID)
	vaultBefore := totalMoney(ctx, t, pool, CollateralVaultID)
	yesSharesBefore := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesBefore := shares(ctx, t, pool, noBuyerID, marketID, "NO")

	if err := SettleMintFill(ctx, pool, fill); err != nil {
		t.Fatalf("SettleMintFill: %v", err)
	}

	yesMoneyAfter := totalMoney(ctx, t, pool, yesBuyerID)
	noMoneyAfter := totalMoney(ctx, t, pool, noBuyerID)
	vaultAfter := totalMoney(ctx, t, pool, CollateralVaultID)
	yesSharesAfter := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesAfter := shares(ctx, t, pool, noBuyerID, marketID, "NO")

	if yesMoneyBefore-yesMoneyAfter != yesPaid {
		t.Errorf("yes buyer money: expected -%d, got %d", yesPaid, yesMoneyBefore-yesMoneyAfter)
	}
	if noMoneyBefore-noMoneyAfter != noPaid {
		t.Errorf("no buyer money: expected -%d, got %d", noPaid, noMoneyBefore-noMoneyAfter)
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
		t.Errorf("vault invariant: expected %d, got %d", PairTotal*qty, vaultAfter-vaultBefore)
	}
}

func mustDepositAndLock(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, amount int64) {
	if err := ledger.Deposit(ctx, pool, userID, amount); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := ledger.Lock(ctx, pool, userID, amount); err != nil {
		t.Fatalf("lock: %v", err)
	}
}
