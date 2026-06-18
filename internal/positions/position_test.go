package positions

import (
	"context"
	"math/rand"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestShareConservation(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, "DELETE FROM share_ledger")
	_, _ = pool.Exec(ctx, "DELETE FROM positions")

	userIDs := []int64{58, 59, 60, 61, 62}

	type marketOutcome struct {
		market  int64
		outcome string
	}
	markets := []marketOutcome{
		{7, "YES"},
		{7, "NO"},
		{12, "YES"},
	}

	expected := make(map[marketOutcome]int64)

	actualTotal := func(mo marketOutcome) int64 {
		var total int64
		err := pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(quantity), 0) FROM positions WHERE market_id=$1 AND outcome=$2",
			mo.market, mo.outcome).Scan(&total)
		if err != nil {
			t.Fatalf("failed to read total: %v", err)
		}
		return total
	}

	var nCredit, nDebitTried, nDebitDone, nTxTried, nTxDone int

	for i := 0; i < 1000; i++ {
		mo := markets[rand.Intn(len(markets))]
		op := rand.Intn(3)

		switch op {
		case 0:
			id := userIDs[rand.Intn(len(userIDs))]
			qty := int64(rand.Intn(100) + 1)
			if err := CreditShares(ctx, pool, id, mo.market, mo.outcome, qty); err != nil {
				t.Fatalf("iteration %d: credit failed: %v", i, err)
			}
			expected[mo] += qty
			nCredit++

		case 1:
			id := userIDs[rand.Intn(len(userIDs))]
			nDebitTried++
			have := userShares(ctx, t, pool, id, mo.market, mo.outcome)
			if have > 0 {
				qty := int64(rand.Int63n(have)) + 1
				if err := DebitShares(ctx, pool, id, mo.market, mo.outcome, qty); err == nil {
					expected[mo] -= qty
					nDebitDone++
				}
			}

		case 2:
			from := userIDs[rand.Intn(len(userIDs))]
			to := userIDs[rand.Intn(len(userIDs))]
			if from == to {
				break
			}
			nTxTried++
			have := userShares(ctx, t, pool, from, mo.market, mo.outcome)
			if have > 0 {
				qty := int64(rand.Int63n(have)) + 1
				if err := TransferShares(ctx, pool, from, to, mo.market, mo.outcome, qty); err == nil {
					nTxDone++
				}
			}
		}

		if got := actualTotal(mo); got != expected[mo] {
			t.Fatalf("iteration %d: SHARE CONSERVATION BROKEN for %v — expected %d, got %d",
				i, mo, expected[mo], got)
		}
	}

	t.Logf("credit=%d | debit tried=%d done=%d | transfer tried=%d done=%d",
		nCredit, nDebitTried, nDebitDone, nTxTried, nTxDone)
}

func userShares(ctx context.Context, t *testing.T, pool *pgxpool.Pool, userID, marketID int64, outcome string) int64 {
	var qty int64
	err := pool.QueryRow(ctx,
		"SELECT COALESCE(quantity, 0) FROM positions WHERE user_id=$1 AND market_id=$2 AND outcome=$3",
		userID, marketID, outcome).Scan(&qty)
	if err != nil {
		return 0
	}
	return qty
}
