package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/fyodor/messenger/chat-service/internal/config"
	"github.com/fyodor/messenger/chat-service/internal/handler"
	"github.com/fyodor/messenger/chat-service/internal/hub"
	"github.com/fyodor/messenger/chat-service/internal/kafka"
	"github.com/fyodor/messenger/chat-service/internal/repository"
	"github.com/fyodor/messenger/chat-service/internal/service"
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

	wsHub := hub.New()
	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	userSvc := service.NewUserService(repo.AsUserRepo())
	roomSvc := service.NewRoomService(repo.AsRoomRepo())
	dmSvc := service.NewDMService(repo.AsDMRepo(), repo.AsUserRepo())
	msgSvc := service.NewMessageService(repo.AsMessageRepo(), repo.AsRoomRepo(), wsHub)

	firstBroker := strings.Split(cfg.KafkaBrokers, ",")[0]
	h := handler.New(userSvc, roomSvc, dmSvc, msgSvc, wsHub, producer, cfg.JWTSecret, firstBroker)

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

	// Internal endpoint — no auth required.
	r.Post("/internal/broadcast", h.InternalBroadcast)

	// Public WebSocket — auth via query param.
	r.Get("/ws", h.WebSocket)

	// Authenticated REST API.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		r.Get("/api/v1/users", h.ListUsers)
		r.Get("/api/v1/users/me", h.Me)

		r.Post("/api/v1/rooms", h.CreateRoom)
		r.Get("/api/v1/rooms", h.ListRooms)
		r.Get("/api/v1/rooms/me", h.ListJoinedRooms)
		r.Post("/api/v1/rooms/{id}/join", h.JoinRoom)
		r.Post("/api/v1/rooms/{id}/leave", h.LeaveRoom)
		r.Get("/api/v1/rooms/{id}/messages", h.ListMessages)

		r.Post("/api/v1/dms", h.CreateOrGetDM)
		r.Get("/api/v1/dms", h.ListDMs)
	})

	l.Info("chat-service listening", zap.String("port", cfg.Port))
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		l.Fatal("server error", zap.Error(err))
	}
}
