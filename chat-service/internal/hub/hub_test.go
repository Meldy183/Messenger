package hub

import (
	"encoding/json"
	"testing"

	"github.com/fyodor/messenger/chat-service/internal/domain"
)

// newTestClient creates a Client with a buffered Send channel; no WebSocket needed.
func newTestClient(userID string) *Client {
	return &Client{
		UserID:   userID,
		Username: userID,
		Send:     make(chan []byte, 32),
	}
}

func sampleMessage(roomID string) *domain.Message {
	return &domain.Message{
		ID:             "msg-1",
		RoomID:         roomID,
		SenderID:       "u1",
		SenderUsername: "alice",
		Content:        "hello",
	}
}

// drainOne reads exactly one frame from the client's Send channel without blocking.
// Returns (frame, true) if a message was present, ("", false) otherwise.
func drainOne(c *Client) ([]byte, bool) {
	select {
	case b := <-c.Send:
		return b, true
	default:
		return nil, false
	}
}

// TestSubscribeAndBroadcast verifies that a subscribed client receives the broadcast.
func TestSubscribeAndBroadcast(t *testing.T) {
	h := New()
	c := newTestClient("u1")

	h.Subscribe("room-a", c)
	h.Broadcast("room-a", sampleMessage("room-a"))

	frame, ok := drainOne(c)
	if !ok {
		t.Fatal("client did not receive any message after Broadcast")
	}

	var out outboundFrame
	if err := json.Unmarshal(frame, &out); err != nil {
		t.Fatalf("invalid frame JSON: %v", err)
	}
	if out.Type != "new_message" {
		t.Fatalf("expected type 'new_message', got %q", out.Type)
	}
	if out.Message.Content != "hello" {
		t.Fatalf("expected content 'hello', got %q", out.Message.Content)
	}
}

// TestBroadcastNoSubscribers verifies no panic when room has no clients.
func TestBroadcastNoSubscribers(t *testing.T) {
	h := New()
	// Should not panic even with zero subscribers.
	h.Broadcast("empty-room", sampleMessage("empty-room"))
}

// TestUnsubscribe verifies that after Unsubscribe the client stops receiving messages.
func TestUnsubscribe(t *testing.T) {
	h := New()
	c := newTestClient("u1")

	h.Subscribe("room-a", c)
	h.Unsubscribe(c)
	h.Broadcast("room-a", sampleMessage("room-a"))

	if _, ok := drainOne(c); ok {
		t.Fatal("client should not receive messages after Unsubscribe")
	}
}

// TestMultipleClientsAllReceive verifies every subscribed client receives the broadcast.
func TestMultipleClientsAllReceive(t *testing.T) {
	h := New()
	c1 := newTestClient("u1")
	c2 := newTestClient("u2")
	c3 := newTestClient("u3")

	for _, c := range []*Client{c1, c2, c3} {
		h.Subscribe("room-a", c)
	}

	h.Broadcast("room-a", sampleMessage("room-a"))

	for i, c := range []*Client{c1, c2, c3} {
		if _, ok := drainOne(c); !ok {
			t.Fatalf("client %d did not receive the broadcast", i+1)
		}
	}
}

// TestBroadcastIsolatedByRoom verifies that a broadcast to room A doesn't reach room B clients.
func TestBroadcastIsolatedByRoom(t *testing.T) {
	h := New()
	cA := newTestClient("u1")
	cB := newTestClient("u2")

	h.Subscribe("room-a", cA)
	h.Subscribe("room-b", cB)

	h.Broadcast("room-a", sampleMessage("room-a"))

	if _, ok := drainOne(cA); !ok {
		t.Fatal("client in room-a did not receive the broadcast")
	}
	if _, ok := drainOne(cB); ok {
		t.Fatal("client in room-b should NOT receive a broadcast for room-a")
	}
}
