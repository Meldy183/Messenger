package repository

import (
	"context"

	"github.com/fyodor/messenger/auth-service/internal/domain"
)

// Repository is the data-access contract for the auth service.
// The service layer depends on this interface, not on the concrete Postgres struct.
// This makes the service testable without a real database.
type Repository interface {
	// CreateUser inserts a new user and returns the persisted record.
	// Returns ErrUsernameTaken if the username already exists.
	CreateUser(ctx context.Context, username, passwordHash string) (*domain.User, error)

	// GetByUsername fetches a user by username.
	// Returns ErrNotFound if no such user exists.
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
}
