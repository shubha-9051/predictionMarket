package ledger

import (
	"context"
	"math/rand"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func TestConservation(t *testing.T) {
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	userIDs := []int64{58, 59, 60, 61, 62}

	var baseline int64
	err = pool.QueryRow(ctx,
		"SELECT COALESCE(SUM(available + locked), 0) FROM users WHERE id = ANY($1)",
		userIDs).Scan(&baseline)
	if err != nil {
		t.Fatalf("failed to read baseline: %v", err)
	}

	var expectedTotal int64 = 0

	var nDeposit, nLockTried, nLockFunded, nRelTried, nRelFunded, nTxTried, nTxFunded int

	for i := 0; i < 1000; i++ {
		op := rand.Intn(4)

		switch op {
		case 0:
			id := userIDs[rand.Intn(len(userIDs))]
			amount := int64(rand.Intn(1000) + 1)
			if err := Deposit(ctx, pool, id, amount); err != nil {
				t.Fatalf("iteration %d: deposit failed: %v", i, err)
			}
			expectedTotal += amount
			nDeposit++

		case 1:
			id := userIDs[rand.Intn(len(userIDs))]
			nLockTried++
			var avail int64
			if err := pool.QueryRow(ctx,
				"SELECT available FROM users WHERE id=$1", id).Scan(&avail); err != nil {
				t.Fatalf("iteration %d: read available failed: %v", i, err)
			}
			if avail > 0 {
				nLockFunded++
				_ = Lock(ctx, pool, id, rand.Int63n(avail)+1)
			}

		case 2:
			id := userIDs[rand.Intn(len(userIDs))]
			nRelTried++
			var locked int64
			if err := pool.QueryRow(ctx,
				"SELECT locked FROM users WHERE id=$1", id).Scan(&locked); err != nil {
				t.Fatalf("iteration %d: read locked failed: %v", i, err)
			}
			if locked > 0 {
				nRelFunded++
				_ = Release(ctx, pool, id, rand.Int63n(locked)+1)
			}

		case 3:
			from := userIDs[rand.Intn(len(userIDs))]
			to := userIDs[rand.Intn(len(userIDs))]
			if from == to {
				break
			}
			nTxTried++
			var locked int64
			if err := pool.QueryRow(ctx,
				"SELECT locked FROM users WHERE id=$1", from).Scan(&locked); err != nil {
				t.Fatalf("iteration %d: read locked failed: %v", i, err)
			}
			if locked > 0 {
				nTxFunded++
				_ = Transfer(ctx, pool, from, to, rand.Int63n(locked)+1)
			}
		}

		var actualTotal int64
		if err := pool.QueryRow(ctx,
			"SELECT COALESCE(SUM(available + locked), 0) FROM users WHERE id = ANY($1)",
			userIDs).Scan(&actualTotal); err != nil {
			t.Fatalf("iteration %d: failed to read total: %v", i, err)
		}
		if actualTotal != expectedTotal+baseline {
			t.Fatalf("iteration %d: CONSERVATION BROKEN — expected %d, got %d",
				i, expectedTotal+baseline, actualTotal)
		}
	}

	t.Logf("deposits=%d | lock tried=%d funded=%d | release tried=%d funded=%d | transfer tried=%d funded=%d",
		nDeposit, nLockTried, nLockFunded, nRelTried, nRelFunded, nTxTried, nTxFunded)
}
