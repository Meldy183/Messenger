package service

import (
	"context"
	"errors"
	"testing"

	"github.com/fyodor/messenger/chat-service/internal/domain"
)

func TestMessageSend(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		msg := &domain.Message{ID: "msg1", RoomID: "r1", SenderID: "u1", Content: "hello"}
		roomRepo := &mockRoomRepo{
			isMemberFn: func(ctx context.Context, roomID, userID string) (bool, error) {
				return true, nil
			},
		}
		msgRepo := &mockMessageRepo{
			createFn: func(ctx context.Context, id, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
				return msg, nil
			},
		}
		broadcaster := &mockBroadcaster{}
		svc := NewMessageService(msgRepo, roomRepo, broadcaster)
		result, err := svc.Send(ctx, "r1", "u1", "alice", "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result == nil || result.ID != "msg1" {
			t.Fatalf("expected message msg1, got %v", result)
		}
	})

	t.Run("not a member", func(t *testing.T) {
		roomRepo := &mockRoomRepo{
			isMemberFn: func(ctx context.Context, roomID, userID string) (bool, error) {
				return false, nil
			},
		}
		svc := NewMessageService(&mockMessageRepo{}, roomRepo, &mockBroadcaster{})
		_, err := svc.Send(ctx, "r1", "u1", "alice", "hello")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got: %v", err)
		}
	})

	t.Run("broadcast is called", func(t *testing.T) {
		msg := &domain.Message{ID: "msg2", RoomID: "r1"}
		roomRepo := &mockRoomRepo{
			isMemberFn: func(ctx context.Context, roomID, userID string) (bool, error) {
				return true, nil
			},
		}
		msgRepo := &mockMessageRepo{
			createFn: func(ctx context.Context, id, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
				return msg, nil
			},
		}

		var capturedRoomID string
		var capturedMsgID string
		broadcaster := &mockBroadcaster{
			broadcastFn: func(roomID string, m *domain.Message) {
				capturedRoomID = roomID
				capturedMsgID = m.ID
			},
		}

		svc := NewMessageService(msgRepo, roomRepo, broadcaster)
		result, err := svc.Send(ctx, "r1", "u1", "alice", "hi")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedRoomID != "r1" {
			t.Fatalf("expected broadcast roomID r1, got %s", capturedRoomID)
		}
		if capturedMsgID != result.ID {
			t.Fatalf("expected broadcast msg ID %s, got %s", result.ID, capturedMsgID)
		}
	})
}

func TestMessageHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		roomRepo := &mockRoomRepo{
			isMemberFn: func(ctx context.Context, roomID, userID string) (bool, error) {
				return true, nil
			},
		}
		msgRepo := &mockMessageRepo{
			listByRoomFn: func(ctx context.Context, roomID string, limit, offset int) ([]*domain.Message, error) {
				return []*domain.Message{
					{ID: "m1", RoomID: roomID},
					{ID: "m2", RoomID: roomID},
				}, nil
			},
		}
		svc := NewMessageService(msgRepo, roomRepo, &mockBroadcaster{})
		msgs, err := svc.History(ctx, "r1", "u1", 50, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(msgs) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(msgs))
		}
	})

	t.Run("not a member", func(t *testing.T) {
		roomRepo := &mockRoomRepo{
			isMemberFn: func(ctx context.Context, roomID, userID string) (bool, error) {
				return false, nil
			},
		}
		svc := NewMessageService(&mockMessageRepo{}, roomRepo, &mockBroadcaster{})
		_, err := svc.History(ctx, "r1", "u1", 50, 0)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got: %v", err)
		}
	})
}
