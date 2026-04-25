package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

const (
	insertQuery = `INSERT INTO users \(id, username, password, created_at\) VALUES \(\$1, \$2, \$3, \$4\) RETURNING id, username, password, created_at`
	selectQuery = `SELECT id, username, password, created_at FROM users WHERE username = \$1`
)

func newTestRepo(t *testing.T) (*PostgresRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewPostgresRepository(db), mock
}

func TestCreateUser_Success(t *testing.T) {
	t.Run("returns persisted user on successful insert", func(t *testing.T) {
		repo, mock := newTestRepo(t)

		id := "550e8400-e29b-41d4-a716-446655440000"
		username := "fyodor"
		hash := "hashed"
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "username", "password", "created_at"}).
			AddRow(id, username, hash, now)

		mock.ExpectQuery(insertQuery).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(rows)

		user, err := repo.CreateUser(context.Background(), username, hash)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Username != username {
			t.Errorf("want Username %q, got %q", username, user.Username)
		}
		if user.PasswordHash != hash {
			t.Errorf("want PasswordHash %q, got %q", hash, user.PasswordHash)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})
}

func TestCreateUser_UsernameTaken(t *testing.T) {
	t.Run("returns ErrUsernameTaken on unique violation", func(t *testing.T) {
		repo, mock := newTestRepo(t)

		mock.ExpectQuery(insertQuery).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(&pq.Error{Code: "23505"})

		_, err := repo.CreateUser(context.Background(), "fyodor", "hashed")
		if err != ErrUsernameTaken {
			t.Errorf("want ErrUsernameTaken, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})
}

func TestGetByUsername_Success(t *testing.T) {
	t.Run("returns user when found", func(t *testing.T) {
		repo, mock := newTestRepo(t)

		id := "550e8400-e29b-41d4-a716-446655440000"
		username := "fyodor"
		hash := "hashed"
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "username", "password", "created_at"}).
			AddRow(id, username, hash, now)

		mock.ExpectQuery(selectQuery).
			WithArgs(username).
			WillReturnRows(rows)

		user, err := repo.GetByUsername(context.Background(), username)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != id {
			t.Errorf("want ID %q, got %q", id, user.ID)
		}
		if user.Username != username {
			t.Errorf("want Username %q, got %q", username, user.Username)
		}
		if user.PasswordHash != hash {
			t.Errorf("want PasswordHash %q, got %q", hash, user.PasswordHash)
		}
		if !user.CreatedAt.Equal(now) {
			t.Errorf("want CreatedAt %v, got %v", now, user.CreatedAt)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})
}

func TestGetByUsername_NotFound(t *testing.T) {
	t.Run("returns ErrNotFound when user does not exist", func(t *testing.T) {
		repo, mock := newTestRepo(t)

		mock.ExpectQuery(selectQuery).
			WithArgs("fyodor").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetByUsername(context.Background(), "fyodor")
		if err != ErrNotFound {
			t.Errorf("want ErrNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled mock expectations: %v", err)
		}
	})
}
