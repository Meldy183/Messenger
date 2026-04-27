package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/fyodor/messenger/auth-service/internal/config"
	"github.com/fyodor/messenger/auth-service/internal/handler"
	"github.com/fyodor/messenger/auth-service/internal/repository"
	"github.com/fyodor/messenger/auth-service/internal/service"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/middleware"
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

	repo := repository.NewPostgresRepository(db)
	svc := service.New(repo, cfg.JWTSecret)
	h := handler.New(svc)

	r := chi.NewRouter()
	r.Use(middleware.Logger(l))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "database not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/api/v1/auth/register", h.Register)
	r.Post("/api/v1/auth/login", h.Login)

	l.Info("auth-service listening", zap.String("port", cfg.Port))
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		l.Fatal("server error", zap.Error(err))
	}
}
