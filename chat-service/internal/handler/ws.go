package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/fyodor/messenger/chat-service/internal/hub"
	kafkaclient "github.com/fyodor/messenger/chat-service/internal/kafka"
	"github.com/fyodor/messenger/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // open for local dev
}

type inboundFrame struct {
	Type    string `json:"type"`
	RoomID  string `json:"room_id"`
	Content string `json:"content"`
}

// WebSocket handles WS /ws — auth via ?token=<jwt> query param.
func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	userID, username, err := h.parseWSToken(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := hub.NewClient(userID, username, conn)
	h.hub.RegisterClient(client)
	defer h.hub.Unsubscribe(client)

	// Subscribe to all joined public rooms.
	rooms, err := h.roomSvc.ListJoined(r.Context(), userID)
	if err == nil {
		for _, rm := range rooms {
			h.hub.Subscribe(rm.ID, client)
		}
	}
	// Subscribe to DM rooms.
	dms, err := h.dmSvc.List(r.Context(), userID)
	if err == nil {
		for _, dm := range dms {
			h.hub.Subscribe(dm.Room.ID, client)
		}
	}

	h.readPump(conn, client, r)
}

// readPump runs in the calling goroutine; blocks until the connection is closed.
func (h *Handler) readPump(conn *websocket.Conn, client *hub.Client, r *http.Request) {
	conn.SetReadLimit(hub.MaxMsgSize)
	conn.SetReadDeadline(time.Now().Add(hub.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(hub.PongWait))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var frame inboundFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			h.sendError(client, "invalid message format")
			continue
		}

		switch frame.Type {
		case "send_message":
			h.handleSendMessage(r, client, frame)
		default:
			h.sendError(client, fmt.Sprintf("unknown message type: %s", frame.Type))
		}
	}
}

func (h *Handler) handleSendMessage(r *http.Request, client *hub.Client, frame inboundFrame) {
	if frame.Content == "" {
		h.sendError(client, "content is required")
		return
	}
	if _, err := uuid.Parse(frame.RoomID); frame.RoomID == "" || err != nil {
		h.sendError(client, "invalid room_id")
		return
	}

	ok, err := h.roomSvc.IsMember(r.Context(), frame.RoomID, client.UserID)
	if err != nil {
		logger.L(r.Context()).Error("ws IsMember check failed", zap.Error(err))
		h.sendError(client, "internal error")
		return
	}
	if !ok {
		h.sendError(client, "not a member of this room")
		return
	}

	msg := kafkaclient.Message{
		ID:             uuid.NewString(),
		RoomID:         frame.RoomID,
		SenderID:       client.UserID,
		SenderUsername: client.Username,
		Content:        frame.Content,
	}
	if err := h.producer.Produce(r.Context(), msg); err != nil {
		logger.L(r.Context()).Error("kafka produce failed", zap.Error(err))
		h.sendError(client, "failed to send message")
	}
}

func (h *Handler) sendError(client *hub.Client, msg string) {
	b, _ := json.Marshal(map[string]string{"type": "error", "message": msg})
	select {
	case client.Send <- b:
	default:
	}
}

func (h *Handler) parseWSToken(r *http.Request) (userID, username string, err error) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		return "", "", fmt.Errorf("missing token")
	}

	var claims jwt.RegisteredClaims
	_, err = jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return h.jwtSecret, nil
	})
	if err != nil || claims.Subject == "" {
		return "", "", fmt.Errorf("invalid token")
	}

	// Fetch username — needed for the Kafka message payload.
	user, err := h.userSvc.GetByID(r.Context(), claims.Subject)
	if err != nil {
		return "", "", fmt.Errorf("user not found: %w", err)
	}
	return claims.Subject, user.Username, nil
}
