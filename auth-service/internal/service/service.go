package service

import (
	"context"
	"errors"

	"github.com/fyodor/messenger/auth-service/internal/domain"
)

var (
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type Service interface {
	// Register creates a new user. Password is hashed before storage.
	// Returns ErrUsernameTaken if the username is already in use.
	Register(ctx context.Context, username, password string) (*domain.User, error)

	// Login validates credentials and returns a signed JWT on success.
	// Returns ErrInvalidCredentials for both wrong username and wrong password
	// (do not leak which one failed).
	Login(ctx context.Context, username, password string) (string, error)
}
