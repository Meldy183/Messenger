package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/service"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/middleware"
	"github.com/fyodor/messenger/pkg/response"
)

type messageResponse struct {
	ID             string    `json:"id"`
	RoomID         string    `json:"room_id"`
	SenderID       string    `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func toMessageResponse(m *domain.Message) messageResponse {
	return messageResponse{
		ID:             m.ID,
		RoomID:         m.RoomID,
		SenderID:       m.SenderID,
		SenderUsername: m.SenderUsername,
		Content:        m.Content,
		CreatedAt:      m.CreatedAt,
	}
}

// ListMessages handles GET /api/v1/rooms/:id/messages?limit=50&offset=0
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")
	userID, _ := middleware.UserIDFromContext(r.Context())

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	msgs, err := h.msgSvc.History(r.Context(), roomID, userID, limit, offset)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			response.Err(w, http.StatusForbidden, "not a member of this room")
			return
		}
		logger.L(r.Context()).Error("list messages failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]messageResponse, len(msgs))
	for i, m := range msgs {
		out[i] = toMessageResponse(m)
	}
	response.JSON(w, http.StatusOK, out)
}

// InternalBroadcast handles POST /internal/broadcast (called by message-worker).
// No auth required — this endpoint is internal only (network policy in k8s).
func (h *Handler) InternalBroadcast(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		RoomID  string          `json:"room_id"`
		Message messageResponse `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid body")
		return
	}

	msg := &domain.Message{
		ID:             payload.Message.ID,
		RoomID:         payload.Message.RoomID,
		SenderID:       payload.Message.SenderID,
		SenderUsername: payload.Message.SenderUsername,
		Content:        payload.Message.Content,
		CreatedAt:      payload.Message.CreatedAt,
	}
	h.hub.Broadcast(payload.RoomID, msg)
	w.WriteHeader(http.StatusNoContent)
}
