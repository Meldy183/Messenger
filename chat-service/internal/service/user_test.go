package service

import (
	"context"
	"errors"
	"testing"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/repository"
)

func TestUserList(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockUserRepo{
			listFn: func(ctx context.Context) ([]*domain.User, error) {
				return []*domain.User{
					{ID: "u1", Username: "alice"},
					{ID: "u2", Username: "bob"},
				}, nil
			},
		}
		svc := NewUserService(repo)
		users, err := svc.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(users))
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repoErr := errors.New("db error")
		repo := &mockUserRepo{
			listFn: func(ctx context.Context) ([]*domain.User, error) {
				return nil, repoErr
			},
		}
		svc := NewUserService(repo)
		_, err := svc.List(ctx)
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected repo error to be propagated, got: %v", err)
		}
	})
}

func TestUserGetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
				return &domain.User{ID: id, Username: "alice"}, nil
			},
		}
		svc := NewUserService(repo)
		user, err := svc.GetByID(ctx, "u1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != "u1" {
			t.Fatalf("expected user ID u1, got %s", user.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
				return nil, repository.ErrNotFound
			},
		}
		svc := NewUserService(repo)
		_, err := svc.GetByID(ctx, "missing")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got: %v", err)
		}
	})
}
