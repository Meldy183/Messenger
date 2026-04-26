package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/fyodor/messenger/message-worker/internal/config"
	"github.com/fyodor/messenger/message-worker/internal/worker"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	l := logger.Must(cfg.Env)
	defer l.Sync()

	db, err := postgres.New(cfg.DBDSN)
	if err != nil {
		l.Fatal("database connection failed", zap.Error(err))
	}
	defer db.Close()

	w := worker.New(cfg.KafkaBrokers, cfg.KafkaGroupID, db, cfg.ChatServiceURL, l)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	l.Info("message-worker started")
	if err := w.Run(ctx); err != nil {
		l.Fatal("worker error", zap.Error(err))
	}
	l.Info("message-worker stopped")
}
