package hub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/fyodor/messenger/chat-service/internal/domain"
)

const (
	writeWait  = 10 * time.Second
	PongWait   = 60 * time.Second
	pingPeriod = (PongWait * 9) / 10
	MaxMsgSize = 4096
)

// Hub manages WebSocket clients grouped by room and indexed by userID for mid-session updates.
type Hub struct {
	mu      sync.RWMutex
	rooms   map[string]map[*Client]struct{} // roomID → clients
	byUser  map[string]*Client              // userID → active client (at most one WS session per user)
}

func New() *Hub {
	return &Hub{
		rooms:  make(map[string]map[*Client]struct{}),
		byUser: make(map[string]*Client),
	}
}

func (h *Hub) Subscribe(roomID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}
}

// SubscribeUser subscribes the currently connected client for userID to roomID, if
// they have an active WebSocket session. Called by the REST join handler so that
// mid-session joins immediately receive real-time messages.
func (h *Hub) SubscribeUser(roomID, userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.byUser[userID]
	if !ok {
		return // user has no active WS session — nothing to do
	}
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}
}

// RegisterClient records the client as the active session for its userID.
// Must be called after NewClient, before readPump.
func (h *Hub) RegisterClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byUser[c.UserID] = c
}

func (h *Hub) Unsubscribe(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, clients := range h.rooms {
		delete(clients, c)
	}
	delete(h.byUser, c.UserID)
}

// Broadcast implements service.Broadcaster — pushes a message to all clients in the room.
func (h *Hub) Broadcast(roomID string, msg *domain.Message) {
	frame, err := json.Marshal(outboundFrame{Type: "new_message", Message: toMsgPayload(msg)})
	if err != nil {
		return
	}
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	for c := range clients {
		select {
		case c.Send <- frame:
		default:
		}
	}
}

// Client is a single WebSocket connection.
type Client struct {
	UserID   string
	Username string
	Send     chan []byte
}

// NewClient creates a client and starts its write pump goroutine.
func NewClient(userID, username string, conn *websocket.Conn) *Client {
	c := &Client{
		UserID:   userID,
		Username: username,
		Send:     make(chan []byte, 256),
	}
	go c.writePump(conn)
	return c
}

func (c *Client) writePump(conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

type outboundFrame struct {
	Type    string     `json:"type"`
	Message msgPayload `json:"message"`
}

type msgPayload struct {
	ID             string    `json:"id"`
	RoomID         string    `json:"room_id"`
	SenderID       string    `json:"sender_id"`
	SenderUsername string    `json:"sender_username"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

func toMsgPayload(m *domain.Message) msgPayload {
	return msgPayload{
		ID:             m.ID,
		RoomID:         m.RoomID,
		SenderID:       m.SenderID,
		SenderUsername: m.SenderUsername,
		Content:        m.Content,
		CreatedAt:      m.CreatedAt,
	}
}
