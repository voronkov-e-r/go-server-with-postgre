package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	config, err := pgxpool.ParseConfig(
		"postgres://bookapi:secret@localhost:5432/bookapi?sslmode=disable",
	)
	if err != nil {
		log.Fatal(err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("connected to postgres")

	var title string
	pool.QueryRow(ctx, "SELECT title FROM books WHERE author = $1", "Orwell").Scan(&title)
	fmt.Println(title)
}
