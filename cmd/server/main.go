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
	_ = godotenv.Load("../../.env")
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		fmt.Println("DATABASE_URL is not set")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		fmt.Println("failed to create pool:", err)
		os.Exit(1)
	}
	defer pool.Close()

	var result int
	err = pool.QueryRow(context.Background(), "SELECT 1").Scan(&result)
	if err != nil {
		fmt.Println("query failed:", err)
		os.Exit(1)
	}

	fmt.Println("connected, SELECT 1 returned:", result)

	err = ledger.Transfer(context.Background(), pool, 1, 2, 2)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("transation is successful for transfer")
	}
}
