package main

import (
	"context"
	"fmt"
	"os"

	"github.com/shubha-9051/predictionMarket/internal/ledger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Get the connection string from an environment variable.
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		fmt.Println("DATABASE_URL is not set")
		os.Exit(1)
	}

	// 2. Create a connection pool.
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		fmt.Println("failed to create pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 3. Run a trivial query to prove the connection works.
	var result int
	err = pool.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		fmt.Println("query failed:", err)
		os.Exit(1)
	}

	// 4. Success.
	fmt.Println("connected, SELECT 1 returned:", result)

	err = ledger.Transfer(context.Background(), pool, 1, 2, 2)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("transation is successful for transfer")
	}
}
