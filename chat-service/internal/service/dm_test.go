package service

import (
	"context"
	"errors"
	"testing"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/repository"
)

func TestDMCreateOrGet(t *testing.T) {
	ctx := context.Background()

	t.Run("self DM", func(t *testing.T) {
		svc := NewDMService(&mockDMRepo{}, &mockUserRepo{})
		_, err := svc.CreateOrGet(ctx, "u1", "u1")
		if !errors.Is(err, ErrSelfDM) {
			t.Fatalf("expected ErrSelfDM, got: %v", err)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		userRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
				return nil, repository.ErrNotFound
			},
		}
		svc := NewDMService(&mockDMRepo{}, userRepo)
		_, err := svc.CreateOrGet(ctx, "u1", "u2")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("creates new DM", func(t *testing.T) {
		newRoom := &domain.Room{ID: "dm1", IsDM: true}
		otherUser := &domain.User{ID: "u2", Username: "bob"}

		userRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
				return &domain.User{ID: id, Username: "bob"}, nil
			},
		}
		dmRepo := &mockDMRepo{
			getByUsersFn: func(ctx context.Context, user1ID, user2ID string) (*domain.Room, error) {
				return nil, repository.ErrNotFound
			},
			createDMFn: func(ctx context.Context, user1ID, user2ID string) (*domain.Room, error) {
				return newRoom, nil
			},
			getOtherMemberFn: func(ctx context.Context, roomID, myUserID string) (*domain.User, error) {
				return otherUser, nil
			},
		}
		svc := NewDMService(dmRepo, userRepo)
		dmRoom, err := svc.CreateOrGet(ctx, "u1", "u2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dmRoom == nil {
			t.Fatal("expected DmRoom, got nil")
		}
		if dmRoom.Room.ID != "dm1" {
			t.Fatalf("expected room ID dm1, got %s", dmRoom.Room.ID)
		}
		if dmRoom.OtherUser.ID != "u2" {
			t.Fatalf("expected OtherUser u2, got %s", dmRoom.OtherUser.ID)
		}
	})

	t.Run("returns existing DM", func(t *testing.T) {
		existingRoom := &domain.Room{ID: "dm-existing", IsDM: true}
		otherUser := &domain.User{ID: "u2", Username: "bob"}

		userRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
				return &domain.User{ID: id, Username: "bob"}, nil
			},
		}
		dmRepo := &mockDMRepo{
			getByUsersFn: func(ctx context.Context, user1ID, user2ID string) (*domain.Room, error) {
				return existingRoom, nil
			},
			getOtherMemberFn: func(ctx context.Context, roomID, myUserID string) (*domain.User, error) {
				return otherUser, nil
			},
		}
		svc := NewDMService(dmRepo, userRepo)
		dmRoom, err := svc.CreateOrGet(ctx, "u1", "u2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dmRoom == nil {
			t.Fatal("expected DmRoom, got nil")
		}
		if dmRoom.Room.ID != "dm-existing" {
			t.Fatalf("expected room ID dm-existing, got %s", dmRoom.Room.ID)
		}
		if dmRoom.OtherUser.ID != "u2" {
			t.Fatalf("expected OtherUser u2, got %s", dmRoom.OtherUser.ID)
		}
	})
}

func TestDMList(t *testing.T) {
	ctx := context.Background()

	t.Run("success with 2 rooms", func(t *testing.T) {
		rooms := []*domain.Room{
			{ID: "dm1", IsDM: true},
			{ID: "dm2", IsDM: true},
		}
		callCount := 0
		dmRepo := &mockDMRepo{
			listByUserFn: func(ctx context.Context, userID string) ([]*domain.Room, error) {
				return rooms, nil
			},
			getOtherMemberFn: func(ctx context.Context, roomID, myUserID string) (*domain.User, error) {
				callCount++
				return &domain.User{ID: "other", Username: "other"}, nil
			},
		}
		svc := NewDMService(dmRepo, &mockUserRepo{})
		dmRooms, err := svc.List(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dmRooms) != 2 {
			t.Fatalf("expected 2 DmRooms, got %d", len(dmRooms))
		}
		if callCount != 2 {
			t.Fatalf("expected GetOtherMember called 2 times, got %d", callCount)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		dmRepo := &mockDMRepo{
			listByUserFn: func(ctx context.Context, userID string) ([]*domain.Room, error) {
				return []*domain.Room{}, nil
			},
		}
		svc := NewDMService(dmRepo, &mockUserRepo{})
		dmRooms, err := svc.List(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(dmRooms) != 0 {
			t.Fatalf("expected empty list, got %d", len(dmRooms))
		}
	})
}
