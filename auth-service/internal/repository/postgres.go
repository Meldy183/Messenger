package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/fyodor/messenger/auth-service/internal/domain"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func NewDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func (r *PostgresRepository) CreateUser(ctx context.Context, username, passwordHash string) (*domain.User, error) {
	const q = `INSERT INTO users (id, username, password, created_at) VALUES ($1, $2, $3, $4) RETURNING id, username, password, created_at`

	var u domain.User
	err := r.db.QueryRowContext(ctx, q, uuid.NewString(), username, passwordHash, time.Now().UTC()).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		// 23505 is the PostgreSQL unique_violation error code
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}

	return &u, nil
}

func (r *PostgresRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const q = `SELECT id, username, password, created_at FROM users WHERE username = $1`

	var u domain.User
	err := r.db.QueryRowContext(ctx, q, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &u, nil
}
