package repository

import (
	"context"

	"github.com/fyodor/messenger/chat-service/internal/domain"
)

// UserRepository handles user read operations.
// chat-service never writes users — auth-service owns that table.
//
// SQL reference:
//
//	List:    SELECT id, username FROM users ORDER BY username
//	GetByID: SELECT id, username FROM users WHERE id = $1
type UserRepository interface {
	List(ctx context.Context) ([]*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

// RoomRepository handles public room lifecycle and membership.
//
// SQL reference:
//
//	Create (transaction):
//	  INSERT INTO rooms (id, name, is_dm, created_by, created_at) VALUES ($1, $2, false, $3, $4)
//	  RETURNING id, name, is_dm, created_by, created_at
//	  INSERT INTO room_members (room_id, user_id, joined_at) VALUES ($1, $2, $3)
//
//	GetByID:
//	  SELECT id, name, is_dm, created_by, created_at FROM rooms WHERE id = $1
//
//	ListPublic:
//	  SELECT id, name, is_dm, created_by, created_at FROM rooms
//	  WHERE is_dm = false ORDER BY created_at DESC
//
//	ListByMember:
//	  SELECT r.id, r.name, r.is_dm, r.created_by, r.created_at
//	  FROM rooms r JOIN room_members rm ON rm.room_id = r.id
//	  WHERE rm.user_id = $1 AND r.is_dm = false ORDER BY r.created_at DESC
//
//	AddMember:
//	  INSERT INTO room_members (room_id, user_id, joined_at) VALUES ($1, $2, $3)
//	  ON CONFLICT DO NOTHING
//
//	RemoveMember:
//	  DELETE FROM room_members WHERE room_id = $1 AND user_id = $2
//
//	IsMember:
//	  SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)
type RoomRepository interface {
	Create(ctx context.Context, name, createdBy string) (*domain.Room, error)
	GetByID(ctx context.Context, id string) (*domain.Room, error)
	ListPublic(ctx context.Context) ([]*domain.Room, error)
	ListByMember(ctx context.Context, userID string) ([]*domain.Room, error)
	AddMember(ctx context.Context, roomID, userID string) error
	RemoveMember(ctx context.Context, roomID, userID string) error
	IsMember(ctx context.Context, roomID, userID string) (bool, error)
}

// DMRepository handles direct-message room lifecycle.
//
// SQL reference:
//
//	CreateDM (transaction):
//	  INSERT INTO rooms (id, name, is_dm, created_by, created_at) VALUES ($1, NULL, true, $2, $3)
//	  RETURNING id, name, is_dm, created_by, created_at
//	  INSERT INTO room_members (room_id, user_id, joined_at) VALUES ($1, $2, $3)  -- for each of the 2 users
//
//	GetByUsers:
//	  SELECT r.id, r.name, r.is_dm, r.created_by, r.created_at
//	  FROM rooms r
//	  JOIN room_members rm1 ON rm1.room_id = r.id AND rm1.user_id = $1
//	  JOIN room_members rm2 ON rm2.room_id = r.id AND rm2.user_id = $2
//	  WHERE r.is_dm = true LIMIT 1
//
//	ListByUser:
//	  SELECT r.id, r.name, r.is_dm, r.created_by, r.created_at
//	  FROM rooms r JOIN room_members rm ON rm.room_id = r.id
//	  WHERE rm.user_id = $1 AND r.is_dm = true ORDER BY r.created_at DESC
//
//	GetOtherMember:
//	  SELECT u.id, u.username FROM users u
//	  JOIN room_members rm ON rm.user_id = u.id
//	  WHERE rm.room_id = $1 AND u.id != $2 LIMIT 1
type DMRepository interface {
	CreateDM(ctx context.Context, user1ID, user2ID string) (*domain.Room, error)
	GetByUsers(ctx context.Context, user1ID, user2ID string) (*domain.Room, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.Room, error)
	GetOtherMember(ctx context.Context, roomID, myUserID string) (*domain.User, error)
}

// MessageRepository handles message persistence.
//
// SQL reference:
//
//	Create:
//	  INSERT INTO messages (id, room_id, sender_id, sender_username, content, created_at)
//	  VALUES ($1, $2, $3, $4, $5, $6)
//	  RETURNING id, room_id, sender_id, sender_username, content, created_at
//
//	ListByRoom (newest-first, paginated):
//	  SELECT id, room_id, sender_id, sender_username, content, created_at
//	  FROM messages WHERE room_id = $1
//	  ORDER BY created_at DESC LIMIT $2 OFFSET $3
type MessageRepository interface {
	Create(ctx context.Context, id, roomID, senderID, senderUsername, content string) (*domain.Message, error)
	ListByRoom(ctx context.Context, roomID string, limit, offset int) ([]*domain.Message, error)
}
