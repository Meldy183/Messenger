package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/kafka"
	"github.com/fyodor/messenger/chat-service/internal/service"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/middleware"
	"github.com/fyodor/messenger/pkg/response"
)

type dmRoomResponse struct {
	Room      roomResponse `json:"room"`
	OtherUser struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"other_user"`
}

func toDMRoomResponse(dm *domain.DmRoom) dmRoomResponse {
	r := dmRoomResponse{Room: toRoomResponse(dm.Room)}
	if dm.OtherUser != nil {
		r.OtherUser.ID = dm.OtherUser.ID
		r.OtherUser.Username = dm.OtherUser.Username
	}
	return r
}

// CreateOrGetDM handles POST /api/v1/dms
func (h *Handler) CreateOrGetDM(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		response.Err(w, http.StatusBadRequest, "user_id is required")
		return
	}

	requesterID, _ := middleware.UserIDFromContext(r.Context())
	dm, err := h.dmSvc.CreateOrGet(r.Context(), requesterID, req.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSelfDM):
			response.Err(w, http.StatusBadRequest, "cannot create a DM with yourself")
		case errors.Is(err, service.ErrNotFound):
			response.Err(w, http.StatusNotFound, "user not found")
		default:
			logger.L(r.Context()).Error("create DM failed", zap.Error(err))
			response.Err(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	// Provision Kafka topic for this DM room — best-effort, same as public rooms.
	go func() {
		if err := kafka.CreateTopic(h.kafkaBroker, dm.Room.ID); err != nil {
			logger.L(r.Context()).Warn("kafka DM topic creation failed", zap.String("room_id", dm.Room.ID), zap.Error(err))
		}
	}()

	response.JSON(w, http.StatusCreated, toDMRoomResponse(dm))
}

// ListDMs handles GET /api/v1/dms
func (h *Handler) ListDMs(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	dms, err := h.dmSvc.List(r.Context(), userID)
	if err != nil {
		logger.L(r.Context()).Error("list DMs failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	out := make([]dmRoomResponse, len(dms))
	for i, dm := range dms {
		out[i] = toDMRoomResponse(dm)
	}
	response.JSON(w, http.StatusOK, out)
}
