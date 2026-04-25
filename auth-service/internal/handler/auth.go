package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/fyodor/messenger/auth-service/internal/service"
	"github.com/fyodor/messenger/pkg/logger"
	"github.com/fyodor/messenger/pkg/response"
	"go.uber.org/zap"
)

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		response.Err(w, http.StatusBadRequest, "username and password are required")
		return
	}

	user, err := h.svc.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			response.Err(w, http.StatusConflict, "username already taken")
			return
		}
		logger.L(r.Context()).Error("register failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}

	response.JSON(w, http.StatusCreated, userResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		response.Err(w, http.StatusBadRequest, "username and password are required")
		return
	}

	token, err := h.svc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Err(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		logger.L(r.Context()).Error("login failed", zap.Error(err))
		response.Err(w, http.StatusInternalServerError, "internal error")
		return
	}

	response.JSON(w, http.StatusOK, tokenResponse{Token: token})
}
