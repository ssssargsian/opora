// Command migrate applies Opora's embedded image migrations as a one-off process.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fatal("DATABASE_URL is required")
	}
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "/app/migrations"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		fatal("database configuration is invalid")
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		fatal("database is unavailable")
	}
	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		fatal("database migrations failed")
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
