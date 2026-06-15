package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"convert-backend/internal/pkg/config"
	"convert-backend/internal/pkg/logger"
	"convert-backend/internal/worker/consumer"
	"convert-backend/internal/worker/executor"
)

func main() {
	cfg := config.LoadWorker()
	logg := logger.New(cfg.ServiceName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exec := executor.NewNoopExecutor()
	cons := consumer.NewNoopConsumer(exec)

	logg.Info("worker started")
	if err := cons.Run(ctx); err != nil {
		logg.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
	logg.Info("worker stopped")
}
