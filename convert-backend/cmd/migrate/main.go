package main

import (
	"context"
	"flag"
	"log"
	"time"

	"convert-backend/internal/pkg/config"
	"convert-backend/internal/pkg/migrate"

	_ "github.com/lib/pq"
)

func main() {
	dsn := flag.String("dsn", "", "PostgreSQL DSN")
	dir := flag.String("dir", "migrations", "migration directory")
	flag.Parse()

	if *dsn == "" {
		cfg := config.LoadGateway()
		*dsn = cfg.Database.DSN
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := migrate.Up(ctx, *dsn, *dir); err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	log.Printf("migrations applied from %s", *dir)
}
