package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"convert-backend/internal/gateway/handler"
	"convert-backend/internal/gateway/middleware"
	"convert-backend/internal/gateway/repository"
	"convert-backend/internal/gateway/rpcclient"
	"convert-backend/internal/gateway/service"
	"convert-backend/internal/pkg/config"
	"convert-backend/internal/pkg/logger"
	"convert-backend/internal/pkg/queue"
	"convert-backend/internal/pkg/storage"
)

func main() {
	cfg := config.LoadGateway()
	logg := logger.New(cfg.ServiceName)

	repo := repository.NewMemoryRepository()
	store, err := storage.NewS3Client(context.Background(), cfg.Storage)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}
	publisher := queue.NewNoopPublisher()
	processors := rpcclient.NewRegistry(cfg.Processors)
	jobService := service.NewJobService(repo, store, publisher, processors)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, jobService)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware.RequestID(middleware.Recover(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logg.Info("gateway listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	logg.Info("gateway stopped")
}
