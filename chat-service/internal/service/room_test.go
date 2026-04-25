package service

import (
	"context"
	"errors"
	"testing"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/repository"
)

func TestRoomCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		name := "general"
		repo := &mockRoomRepo{
			createFn: func(ctx context.Context, n, createdBy string) (*domain.Room, error) {
				return &domain.Room{ID: "r1", Name: &name}, nil
			},
		}
		svc := NewRoomService(repo)
		room, err := svc.Create(ctx, name, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if room == nil || room.ID != "r1" {
			t.Fatalf("expected room r1, got %v", room)
		}
	})

	t.Run("name taken", func(t *testing.T) {
		repo := &mockRoomRepo{
			createFn: func(ctx context.Context, name, createdBy string) (*domain.Room, error) {
				return nil, repository.ErrRoomNameTaken
			},
		}
		svc := NewRoomService(repo)
		_, err := svc.Create(ctx, "taken", "u1")
		if !errors.Is(err, ErrRoomNameTaken) {
			t.Fatalf("expected ErrRoomNameTaken, got: %v", err)
		}
	})
}

func TestRoomJoin(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		name := "general"
		repo := &mockRoomRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Room, error) {
				return &domain.Room{ID: id, Name: &name, IsDM: false}, nil
			},
			addMemberFn: func(ctx context.Context, roomID, userID string) error {
				return nil
			},
		}
		svc := NewRoomService(repo)
		err := svc.Join(ctx, "r1", "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("room not found", func(t *testing.T) {
		repo := &mockRoomRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Room, error) {
				return nil, repository.ErrNotFound
			},
		}
		svc := NewRoomService(repo)
		err := svc.Join(ctx, "missing", "u1")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("is DM", func(t *testing.T) {
		repo := &mockRoomRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.Room, error) {
				return &domain.Room{ID: id, IsDM: true}, nil
			},
		}
		svc := NewRoomService(repo)
		err := svc.Join(ctx, "dm-room", "u1")
		if !errors.Is(err, ErrIsDMRoom) {
			t.Fatalf("expected ErrIsDMRoom, got: %v", err)
		}
	})
}

func TestRoomLeave(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockRoomRepo{
			removeMemberFn: func(ctx context.Context, roomID, userID string) error {
				return nil
			},
		}
		svc := NewRoomService(repo)
		err := svc.Leave(ctx, "r1", "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("not a member is idempotent", func(t *testing.T) {
		repo := &mockRoomRepo{
			removeMemberFn: func(ctx context.Context, roomID, userID string) error {
				return repository.ErrNotMember
			},
		}
		svc := NewRoomService(repo)
		err := svc.Leave(ctx, "r1", "u1")
		if err != nil {
			t.Fatalf("expected nil error for non-member leave, got: %v", err)
		}
	})
}

func TestRoomListPublic(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		n1, n2 := "alpha", "beta"
		repo := &mockRoomRepo{
			listPublicFn: func(ctx context.Context) ([]*domain.Room, error) {
				return []*domain.Room{
					{ID: "r1", Name: &n1},
					{ID: "r2", Name: &n2},
				}, nil
			},
		}
		svc := NewRoomService(repo)
		rooms, err := svc.ListPublic(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rooms) != 2 {
			t.Fatalf("expected 2 rooms, got %d", len(rooms))
		}
	})
}

func TestRoomListJoined(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		n := "general"
		repo := &mockRoomRepo{
			listByMemberFn: func(ctx context.Context, userID string) ([]*domain.Room, error) {
				return []*domain.Room{
					{ID: "r1", Name: &n},
				}, nil
			},
		}
		svc := NewRoomService(repo)
		rooms, err := svc.ListJoined(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rooms) != 1 {
			t.Fatalf("expected 1 room, got %d", len(rooms))
		}
	})
}
