package placement

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/shubha-9051/predictionMarket/internal/ledger"
	"github.com/shubha-9051/predictionMarket/internal/matching"
	"github.com/shubha-9051/predictionMarket/internal/orders"
	"github.com/shubha-9051/predictionMarket/internal/positions"
	"github.com/shubha-9051/predictionMarket/internal/settlement"
)

func TestPlaceOrderEndToEnd(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	exchange := matching.NewExchange()

	sellerID := createUser(ctx, t, pool)
	buyerID := createUser(ctx, t, pool)

	const (
		marketID = int64(7)
		outcome  = "YES"
		sellPx   = int64(550)
		buyPx    = int64(600)
		qty      = int64(60)
	)

	if err := positions.CreditShares(ctx, pool, sellerID, marketID, outcome, qty); err != nil {
		t.Fatalf("credit seller shares: %v", err)
	}
	sellOrderID, err := orders.CreateOrder(ctx, pool, sellerID, marketID, outcome, "sell", sellPx, qty)
	if err != nil {
		t.Fatalf("create sell order: %v", err)
	}
	sellOrder := matching.Order{
		ID:        sellOrderID,
		Outcome:   "YES",
		Side:      "sell",
		Price:     sellPx,
		Quantity:  qty,
		Remaining: qty,
	}
	matching.Normalize(&sellOrder)

	book := exchange.BookFor(marketID)
	book.Add(sellOrder)

	if err := ledger.Deposit(ctx, pool, buyerID, buyPx*qty); err != nil {
		t.Fatalf("deposit buyer: %v", err)
	}

	buyerMoneyBefore := totalMoney(ctx, t, pool, buyerID)
	sellerMoneyBefore := totalMoney(ctx, t, pool, sellerID)
	buyerSharesBefore := shares(ctx, t, pool, buyerID, marketID, outcome)
	sellerSharesBefore := shares(ctx, t, pool, sellerID, marketID, outcome)

	fills, err := PlaceOrder(ctx, pool, exchange, buyerID, marketID, outcome, "buy", buyPx, qty)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	buyerMoneyAfter := totalMoney(ctx, t, pool, buyerID)
	sellerMoneyAfter := totalMoney(ctx, t, pool, sellerID)
	buyerSharesAfter := shares(ctx, t, pool, buyerID, marketID, outcome)
	sellerSharesAfter := shares(ctx, t, pool, sellerID, marketID, outcome)

	tradeValue := sellPx * qty

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Price != sellPx || fills[0].Quantity != qty {
		t.Errorf("fill expected %d@%d, got %d@%d", qty, sellPx, fills[0].Quantity, fills[0].Price)
	}

	if buyerMoneyBefore-buyerMoneyAfter != tradeValue {
		t.Errorf("buyer money: expected -%d, got %d", tradeValue, buyerMoneyBefore-buyerMoneyAfter)
	}
	if sellerMoneyAfter-sellerMoneyBefore != tradeValue {
		t.Errorf("seller money: expected +%d, got %d", tradeValue, sellerMoneyAfter-sellerMoneyBefore)
	}

	if sellerSharesBefore-sellerSharesAfter != qty {
		t.Errorf("seller shares: expected -%d, got %d", qty, sellerSharesBefore-sellerSharesAfter)
	}
	if buyerSharesAfter-buyerSharesBefore != qty {
		t.Errorf("buyer shares: expected +%d, got %d", qty, buyerSharesAfter-buyerSharesBefore)
	}

	buyOrderDB := getOrder(ctx, t, pool, fills[0].IncomingOrderID)
	sellOrderDB := getOrder(ctx, t, pool, sellOrderID)
	if buyOrderDB.Remaining != 0 || buyOrderDB.Status != "filled" {
		t.Errorf("buy order: expected remaining 0 / filled, got %d / %s", buyOrderDB.Remaining, buyOrderDB.Status)
	}
	if sellOrderDB.Remaining != 0 || sellOrderDB.Status != "filled" {
		t.Errorf("sell order: expected remaining 0 / filled, got %d / %s", sellOrderDB.Remaining, sellOrderDB.Status)
	}

	var locked int64
	if err := pool.QueryRow(ctx, "SELECT locked FROM users WHERE id=$1", buyerID).Scan(&locked); err != nil {
		t.Fatalf("read locked: %v", err)
	}
	if locked != 0 {
		t.Errorf("buyer locked: expected 0 after full fill + over-lock release, got %d", locked)
	}
}

func createUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	var id int64
	if err := pool.QueryRow(ctx,
		"INSERT INTO users (available, locked) VALUES (0,0) RETURNING id").Scan(&id); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func totalMoney(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID int64) int64 {
	var total int64
	if err := pool.QueryRow(ctx,
		"SELECT available + locked FROM users WHERE id=$1", userID).Scan(&total); err != nil {
		t.Fatalf("read money: %v", err)
	}
	return total
}

func shares(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, marketID int64, outcome string) int64 {
	var qty int64
	if err := pool.QueryRow(ctx,
		"SELECT COALESCE(quantity,0) FROM positions WHERE user_id=$1 AND market_id=$2 AND outcome=$3",
		userID, marketID, outcome).Scan(&qty); err != nil {
		return 0
	}
	return qty
}

func getOrder(ctx context.Context, t *testing.T, pool *pgxpool.Pool, id int64) orders.Order {
	o, err := orders.GetOrder(ctx, pool, id)
	if err != nil {
		t.Fatalf("get order %d: %v", id, err)
	}
	return o
}

func TestMintThroughPlacement(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	exchange := matching.NewExchange()

	noBuyerID := createUser(ctx, t, pool)
	yesBuyerID := createUser(ctx, t, pool)

	const (
		marketID = int64(20)
		noPx     = int64(400)
		yesPx    = int64(600)
		qty      = int64(50)
	)

	if err := ledger.Deposit(ctx, pool, noBuyerID, noPx*qty); err != nil {
		t.Fatalf("deposit no buyer: %v", err)
	}
	if err := ledger.Deposit(ctx, pool, yesBuyerID, yesPx*qty); err != nil {
		t.Fatalf("deposit yes buyer: %v", err)
	}

	noFills, err := PlaceOrder(ctx, pool, exchange, noBuyerID, marketID, "NO", "buy", noPx, qty)
	if err != nil {
		t.Fatalf("place NO order: %v", err)
	}
	if len(noFills) != 0 {
		t.Fatalf("expected NO order to rest (0 fills), got %d", len(noFills))
	}

	vaultBefore := totalMoney(ctx, t, pool, settlement.CollateralVaultID)
	yesSharesBefore := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesBefore := shares(ctx, t, pool, noBuyerID, marketID, "NO")

	yesFills, err := PlaceOrder(ctx, pool, exchange, yesBuyerID, marketID, "YES", "buy", yesPx, qty)
	if err != nil {
		t.Fatalf("place YES order: %v", err)
	}

	if len(yesFills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(yesFills))
	}
	if yesFills[0].Type != "mint" {
		t.Errorf("expected mint fill, got type %q", yesFills[0].Type)
	}

	yesSharesAfter := shares(ctx, t, pool, yesBuyerID, marketID, "YES")
	noSharesAfter := shares(ctx, t, pool, noBuyerID, marketID, "NO")
	if yesSharesAfter-yesSharesBefore != qty {
		t.Errorf("yes shares: expected +%d, got %d", qty, yesSharesAfter-yesSharesBefore)
	}
	if noSharesAfter-noSharesBefore != qty {
		t.Errorf("no shares: expected +%d, got %d", qty, noSharesAfter-noSharesBefore)
	}

	vaultAfter := totalMoney(ctx, t, pool, settlement.CollateralVaultID)
	expectedCollateral := matching.PairTotal * qty
	if vaultAfter-vaultBefore != expectedCollateral {
		t.Errorf("vault: expected +%d, got %d", expectedCollateral, vaultAfter-vaultBefore)
	}
}
