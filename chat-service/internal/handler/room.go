package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/kafka"
	"github.com/fyodor/messenger/chat-service/internal/service"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/middleware"
	"github.com/fyodor/messenger/pkg/response"
)

type roomResponse struct {
	ID        string    `json:"id"`
	Name      *string   `json:"name"`
	IsDM      bool      `json:"is_dm"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func toRoomResponse(rm *domain.Room) roomResponse {
	return roomResponse{
		ID:        rm.ID,
		Name:      rm.Name,
		IsDM:      rm.IsDM,
		CreatedBy: rm.CreatedBy,
		CreatedAt: rm.CreatedAt,
	}
}

// CreateRoom handles POST /api/v1/rooms
func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		response.Err(w, http.StatusBadRequest, "name is required")
		return
	}

	userID, _ := middleware.UserIDFromContext(r.Context())
	room, err := h.roomSvc.Create(r.Context(), req.Name, userID)
	if err != nil {
		if errors.Is(err, service.ErrRoomNameTaken) {
			response.Err(w, http.StatusConflict, "room name already taken")
			return
		}
		logger.L(r.Context()).Error("create room failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Provision Kafka topic — best-effort; log failure but don't fail the HTTP response.
	go func() {
		if err := kafka.CreateTopic(h.kafkaBroker, room.ID); err != nil {
			logger.L(r.Context()).Warn("kafka topic creation failed", zap.String("room_id", room.ID), zap.Error(err))
		}
	}()

	response.JSON(w, http.StatusCreated, toRoomResponse(room))
}

// ListRooms handles GET /api/v1/rooms
func (h *Handler) ListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.roomSvc.ListPublic(r.Context())
	if err != nil {
		logger.L(r.Context()).Error("list rooms failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]roomResponse, len(rooms))
	for i, rm := range rooms {
		out[i] = toRoomResponse(rm)
	}
	response.JSON(w, http.StatusOK, out)
}

// ListJoinedRooms handles GET /api/v1/rooms/me
func (h *Handler) ListJoinedRooms(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	rooms, err := h.roomSvc.ListJoined(r.Context(), userID)
	if err != nil {
		logger.L(r.Context()).Error("list joined rooms failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]roomResponse, len(rooms))
	for i, rm := range rooms {
		out[i] = toRoomResponse(rm)
	}
	response.JSON(w, http.StatusOK, out)
}

// JoinRoom handles POST /api/v1/rooms/:id/join
func (h *Handler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")
	userID, _ := middleware.UserIDFromContext(r.Context())

	if err := h.roomSvc.Join(r.Context(), roomID, userID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			response.Err(w, http.StatusNotFound, "room not found")
		case errors.Is(err, service.ErrIsDMRoom):
			response.Err(w, http.StatusForbidden, "cannot join a DM room")
		default:
			logger.L(r.Context()).Error("join room failed", zap.Error(err))
			response.Err(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LeaveRoom handles POST /api/v1/rooms/:id/leave
func (h *Handler) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")
	userID, _ := middleware.UserIDFromContext(r.Context())

	if err := h.roomSvc.Leave(r.Context(), roomID, userID); err != nil {
		logger.L(r.Context()).Error("leave room failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
