package main

import (
	"context"
	"database/sql"
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

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.LoadGateway()
	logg := logger.New(cfg.ServiceName)

	db, err := sql.Open("postgres", cfg.Database.DSN)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	repo := repository.NewPostgresRepository(db)
	store, err := storage.NewS3Client(context.Background(), cfg.Storage)
	if err != nil {
		log.Fatalf("storage init failed: %v", err)
	}
	natsQueue, err := queue.NewNATSClient(context.Background(), cfg.Queue)
	if err != nil {
		log.Fatalf("queue init failed: %v", err)
	}
	defer natsQueue.Close()
	processors := rpcclient.NewRegistry(cfg.Processors)
	jobService := service.NewJobService(
		repo,
		store,
		natsQueue,
		processors,
		service.WithMaxUploadSizeBytes(int64(cfg.Limits.MaxUploadSizeMB)<<20),
	)

	router := handler.NewRouter(jobService, db.PingContext)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           middleware.RequestID(middleware.Recover(router)),
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
