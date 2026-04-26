package service

import (
	"context"
	"errors"

	"github.com/fyodor/messenger/chat-service/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrForbidden     = errors.New("forbidden")
	ErrRoomNameTaken = errors.New("room name already taken")
	ErrNotMember     = errors.New("not a member of this room")
	ErrIsDMRoom      = errors.New("cannot join a DM room")
	ErrSelfDM        = errors.New("cannot create a DM with yourself")
)

// Broadcaster delivers a persisted message to connected WebSocket clients.
// Implemented by the WebSocket hub (Phase 6). A no-op stub is used until then.
type Broadcaster interface {
	Broadcast(roomID string, msg *domain.Message)
}

type UserService interface {
	List(ctx context.Context) ([]*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

type RoomService interface {
	// Create makes a new public room. The creator is automatically joined.
	// Returns ErrRoomNameTaken if the name is already in use.
	Create(ctx context.Context, name, createdByUserID string) (*domain.Room, error)

	// Join adds userID to roomID. Idempotent.
	// Returns ErrNotFound if the room does not exist.
	// Returns ErrIsDMRoom if the room is a DM.
	Join(ctx context.Context, roomID, userID string) error

	// Leave removes userID from roomID. Idempotent — no error if not a member.
	Leave(ctx context.Context, roomID, userID string) error

	ListPublic(ctx context.Context) ([]*domain.Room, error)
	ListJoined(ctx context.Context, userID string) ([]*domain.Room, error)
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
}

type DMService interface {
	// CreateOrGet returns the DM room between requester and target.
	// Creates it if it does not exist. Always returns 201 per the API contract.
	// Returns ErrSelfDM if requesterID == targetUserID.
	// Returns ErrNotFound if targetUserID does not exist.
	CreateOrGet(ctx context.Context, requesterID, targetUserID string) (*domain.DmRoom, error)

	List(ctx context.Context, userID string) ([]*domain.DmRoom, error)
}

type MessageService interface {
	// Send validates membership, persists the message, and broadcasts it.
	// Returns ErrForbidden if the sender is not a member of the room.
	Send(ctx context.Context, roomID, senderID, senderUsername, content string) (*domain.Message, error)

	// History returns messages newest-first. Caller must be a member.
	// Returns ErrForbidden if userID is not a member of the room.
	History(ctx context.Context, roomID, userID string, limit, offset int) ([]*domain.Message, error)
}
