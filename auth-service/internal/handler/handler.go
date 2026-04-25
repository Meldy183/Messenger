package handler

import "github.com/fyodor/messenger/auth-service/internal/service"

// Handler holds the HTTP handlers for the auth service.
// All business logic is delegated to svc — handlers only deal with
// request parsing, error mapping, and response writing.
type Handler struct {
	svc service.Service
}

func New(svc service.Service) *Handler {
	return &Handler{svc: svc}
}
