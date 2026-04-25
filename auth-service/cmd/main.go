package main

import (
	"log"

	"go.uber.org/zap"

	"github.com/fyodor/messenger/auth-service/internal/config"
	"github.com/fyodor/messenger/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	l := logger.Must(cfg.Env)
	defer l.Sync() // flushes any buffered log entries on exit

	l.Info("auth-service starting",
		zap.String("port", cfg.Port),
		zap.String("env", cfg.Env),
	)
}
