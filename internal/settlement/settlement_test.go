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
	"github.com/shubha-9051/predictionMarket/internal/positions"
)

func TestSettleFill(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	buyerID := createUser(ctx, t, pool)
	sellerID := createUser(ctx, t, pool)

	const (
		marketID = int64(7)
		outcome  = "YES"
		price    = int64(550)
		quantity = int64(60)
	)
	moneyAmount := price * quantity

	if err := ledger.Deposit(ctx, pool, buyerID, moneyAmount); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := ledger.Lock(ctx, pool, buyerID, moneyAmount); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := positions.CreditShares(ctx, pool, sellerID, marketID, outcome, quantity); err != nil {
		t.Fatalf("credit shares: %v", err)
	}

	buyOrderID, err := orders.CreateOrder(ctx, pool, buyerID, marketID, outcome, "buy", price, quantity)
	if err != nil {
		t.Fatalf("create buy order: %v", err)
	}
	sellOrderID, err := orders.CreateOrder(ctx, pool, sellerID, marketID, outcome, "sell", price, quantity)
	if err != nil {
		t.Fatalf("create sell order: %v", err)
	}

	buyerMoneyBefore := totalMoney(ctx, t, pool, buyerID)
	sellerMoneyBefore := totalMoney(ctx, t, pool, sellerID)
	buyerSharesBefore := shares(ctx, t, pool, buyerID, marketID, outcome)
	sellerSharesBefore := shares(ctx, t, pool, sellerID, marketID, outcome)

	fill := matching.Fill{
		IncomingOrderID: buyOrderID,
		RestingOrderID:  sellOrderID,
		Price:           price,
		Quantity:        quantity,
	}

	if err := SettleFill(ctx, pool, fill); err != nil {
		t.Fatalf("SettleFill: %v", err)
	}

	buyerMoneyAfter := totalMoney(ctx, t, pool, buyerID)
	sellerMoneyAfter := totalMoney(ctx, t, pool, sellerID)
	buyerSharesAfter := shares(ctx, t, pool, buyerID, marketID, outcome)
	sellerSharesAfter := shares(ctx, t, pool, sellerID, marketID, outcome)

	if buyerMoneyBefore-buyerMoneyAfter != moneyAmount {
		t.Errorf("buyer money: expected -%d, got %d", moneyAmount, buyerMoneyBefore-buyerMoneyAfter)
	}
	if sellerMoneyAfter-sellerMoneyBefore != moneyAmount {
		t.Errorf("seller money: expected +%d, got %d", moneyAmount, sellerMoneyAfter-sellerMoneyBefore)
	}
	if sellerSharesBefore-sellerSharesAfter != quantity {
		t.Errorf("seller shares: expected -%d, got %d", quantity, sellerSharesBefore-sellerSharesAfter)
	}
	if buyerSharesAfter-buyerSharesBefore != quantity {
		t.Errorf("buyer shares: expected +%d, got %d", quantity, buyerSharesAfter-buyerSharesBefore)
	}
}

func createUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	var id int64
	err := pool.QueryRow(ctx,
		"INSERT INTO users (available, locked) VALUES (0,0) RETURNING id").Scan(&id)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return id
}

func totalMoney(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID int64) int64 {
	var total int64
	err := pool.QueryRow(ctx,
		"SELECT available + locked FROM users WHERE id=$1", userID).Scan(&total)
	if err != nil {
		t.Fatalf("read money: %v", err)
	}
	return total
}

func shares(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, marketID int64, outcome string) int64 {
	var qty int64
	err := pool.QueryRow(ctx,
		"SELECT COALESCE(quantity,0) FROM positions WHERE user_id=$1 AND market_id=$2 AND outcome=$3",
		userID, marketID, outcome).Scan(&qty)
	if err != nil {
		return 0
	}
	return qty
}
