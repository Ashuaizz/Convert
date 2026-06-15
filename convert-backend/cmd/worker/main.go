package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"
	"time"

	"convert-backend/internal/pkg/config"
	"convert-backend/internal/pkg/logger"
	"convert-backend/internal/pkg/queue"
	"convert-backend/internal/worker/consumer"
	"convert-backend/internal/worker/executor"

	"convert-backend/internal/gateway/repository"

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.LoadWorker()
	logg := logger.New(cfg.ServiceName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	natsClient, err := queue.NewNATSClient(ctx, cfg.Queue)
	if err != nil {
		logg.Error("queue init failed", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	db, err := sql.Open("postgres", cfg.Database.DSN)
	if err != nil {
		logg.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		logg.Error("database ping failed", "error", err)
		os.Exit(1)
	}

	repo := repository.NewPostgresRepository(db)
	exec := executor.NewStateTransitionExecutor(repo, executor.NewNoopExecutor(), logg)
	cons := consumer.New(natsClient, exec, logg)

	logg.Info("worker started")
	if err := cons.Run(ctx); err != nil {
		logg.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
	logg.Info("worker stopped")
}
