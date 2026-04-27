package handler

import (
	"errors"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/fyodor/messenger/chat-service/internal/service"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/middleware"
	"github.com/fyodor/messenger/pkg/response"
)

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// ListUsers handles GET /api/v1/users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userSvc.List(r.Context())
	if err != nil {
		logger.L(r.Context()).Error("list users failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]userResponse, len(users))
	for i, u := range users {
		out[i] = userResponse{ID: u.ID, Username: u.Username}
	}
	response.JSON(w, http.StatusOK, out)
}

// Me handles GET /api/v1/users/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	user, err := h.userSvc.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			response.Err(w, http.StatusNotFound, "user not found")
			return
		}
		logger.L(r.Context()).Error("get user failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}
	response.JSON(w, http.StatusOK, userResponse{ID: user.ID, Username: user.Username, CreatedAt: user.CreatedAt})
}
