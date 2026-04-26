package handler

import (
	"github.com/fyodor/messenger/chat-service/internal/hub"
	"github.com/fyodor/messenger/chat-service/internal/kafka"
	"github.com/fyodor/messenger/chat-service/internal/service"
)

// Handler holds HTTP and WebSocket handlers for the chat service.
type Handler struct {
	userSvc     service.UserService
	roomSvc     service.RoomService
	dmSvc       service.DMService
	msgSvc      service.MessageService
	hub         *hub.Hub
	producer    *kafka.Producer
	jwtSecret   []byte
	kafkaBroker string // first broker address, used for admin topic creation
}

func New(
	userSvc service.UserService,
	roomSvc service.RoomService,
	dmSvc service.DMService,
	msgSvc service.MessageService,
	h *hub.Hub,
	producer *kafka.Producer,
	jwtSecret string,
	kafkaBroker string,
) *Handler {
	return &Handler{
		userSvc:     userSvc,
		roomSvc:     roomSvc,
		dmSvc:       dmSvc,
		msgSvc:      msgSvc,
		hub:         h,
		producer:    producer,
		jwtSecret:   []byte(jwtSecret),
		kafkaBroker: kafkaBroker,
	}
}
