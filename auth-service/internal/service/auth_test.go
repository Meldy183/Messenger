package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/fyodor/messenger/auth-service/internal/domain"
	"github.com/fyodor/messenger/auth-service/internal/repository"
)

// mockRepo implements repository.Repository for testing without a real database.
type mockRepo struct {
	createUserFn    func(ctx context.Context, username, passwordHash string) (*domain.User, error)
	getByUsernameFn func(ctx context.Context, username string) (*domain.User, error)
}

func (m *mockRepo) CreateUser(ctx context.Context, username, passwordHash string) (*domain.User, error) {
	return m.createUserFn(ctx, username, passwordHash)
}

func (m *mockRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return m.getByUsernameFn(ctx, username)
}

func TestRegister(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		want := &domain.User{
			ID:        "user-1",
			Username:  "alice",
			CreatedAt: time.Now(),
		}
		repo := &mockRepo{
			createUserFn: func(ctx context.Context, username, passwordHash string) (*domain.User, error) {
				return want, nil
			},
		}
		svc := New(repo, "test-secret")

		got, err := svc.Register(context.Background(), "alice", "password")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil user, got nil")
		}
		if got.ID != want.ID {
			t.Errorf("expected user ID %q, got %q", want.ID, got.ID)
		}
	})

	t.Run("UsernameTaken", func(t *testing.T) {
		repo := &mockRepo{
			createUserFn: func(ctx context.Context, username, passwordHash string) (*domain.User, error) {
				return nil, repository.ErrUsernameTaken
			},
		}
		svc := New(repo, "test-secret")

		_, err := svc.Register(context.Background(), "alice", "password")
		if !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("expected ErrUsernameTaken, got: %v", err)
		}
	})
}

func TestLogin(t *testing.T) {
	// Pre-compute a real bcrypt hash once for use across login subtests.
	const plainPassword = "password"
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	newUserWithHash := func(h string) *domain.User {
		return &domain.User{
			ID:           "user-1",
			Username:     "alice",
			PasswordHash: h,
			CreatedAt:    time.Now(),
		}
	}

	t.Run("Success", func(t *testing.T) {
		repo := &mockRepo{
			getByUsernameFn: func(ctx context.Context, username string) (*domain.User, error) {
				return newUserWithHash(string(hash)), nil
			},
		}
		svc := New(repo, "test-secret")

		token, err := svc.Login(context.Background(), "alice", plainPassword)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token, got empty string")
		}
	})

	t.Run("WrongPassword", func(t *testing.T) {
		repo := &mockRepo{
			getByUsernameFn: func(ctx context.Context, username string) (*domain.User, error) {
				return newUserWithHash(string(hash)), nil
			},
		}
		svc := New(repo, "test-secret")

		_, err := svc.Login(context.Background(), "alice", "wrong-password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
		}
	})

	t.Run("UserNotFound", func(t *testing.T) {
		repo := &mockRepo{
			getByUsernameFn: func(ctx context.Context, username string) (*domain.User, error) {
				return nil, repository.ErrNotFound
			},
		}
		svc := New(repo, "test-secret")

		_, err := svc.Login(context.Background(), "ghost", plainPassword)
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
		}
	})
}
