package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PostgresRepository is the single concrete type that satisfies all four repository
// interfaces. Because Go does not allow two methods with the same name on the same
// struct, the conflicting methods (GetByID, Create) are delegated to inner structs
// that each implement exactly one interface. The outer struct embeds those helpers
// and adds the remaining methods directly.
type PostgresRepository struct {
	user *userRepo
	room *roomRepo
	dm   *dmRepo
	msg  *messageRepo
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{
		user: &userRepo{db: db},
		room: &roomRepo{db: db},
		dm:   &dmRepo{db: db},
		msg:  &messageRepo{db: db},
	}
}

// AsUserRepo returns the UserRepository view.
func (r *PostgresRepository) AsUserRepo() UserRepository { return r.user }

// AsRoomRepo returns the RoomRepository view.
func (r *PostgresRepository) AsRoomRepo() RoomRepository { return r.room }

// AsDMRepo returns the DMRepository view.
func (r *PostgresRepository) AsDMRepo() DMRepository { return r.dm }

// AsMessageRepo returns the MessageRepository view.
func (r *PostgresRepository) AsMessageRepo() MessageRepository { return r.msg }

// ---------------------------------------------------------------------------
// userRepo — implements UserRepository
// ---------------------------------------------------------------------------

type userRepo struct{ db *sql.DB }

func (r *userRepo) List(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, username FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(&u.ID, &u.Username); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRowContext(ctx, `SELECT id, username FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// roomRepo — implements RoomRepository
// ---------------------------------------------------------------------------

type roomRepo struct{ db *sql.DB }

func (r *roomRepo) Create(ctx context.Context, name, createdBy string) (_ *domain.Room, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	room := &domain.Room{}
	id := uuid.New().String()
	now := time.Now().UTC()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO rooms (id, name, is_dm, created_by, created_at) VALUES ($1, $2, false, $3, $4) RETURNING id, name, is_dm, created_by, created_at`,
		id, name, createdBy, now,
	).Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			err = ErrRoomNameTaken
		}
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO room_members (room_id, user_id, joined_at) VALUES ($1, $2, $3)`,
		room.ID, createdBy, now,
	)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	return room, err
}

func (r *roomRepo) GetByID(ctx context.Context, id string) (*domain.Room, error) {
	room := &domain.Room{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, is_dm, created_by, created_at FROM rooms WHERE id = $1`, id,
	).Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return room, nil
}

func (r *roomRepo) ListPublic(ctx context.Context) ([]*domain.Room, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, is_dm, created_by, created_at FROM rooms WHERE is_dm = false ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		if err := rows.Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (r *roomRepo) ListByMember(ctx context.Context, userID string) ([]*domain.Room, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT r.id, r.name, r.is_dm, r.created_by, r.created_at FROM rooms r JOIN room_members rm ON rm.room_id = r.id WHERE rm.user_id = $1 AND r.is_dm = false ORDER BY r.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		if err := rows.Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (r *roomRepo) AddMember(ctx context.Context, roomID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO room_members (room_id, user_id, joined_at) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		roomID, userID, time.Now().UTC(),
	)
	return err
}

func (r *roomRepo) RemoveMember(ctx context.Context, roomID, userID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM room_members WHERE room_id = $1 AND user_id = $2`,
		roomID, userID,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotMember
	}
	return nil
}

func (r *roomRepo) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2)`,
		roomID, userID,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// dmRepo — implements DMRepository
// ---------------------------------------------------------------------------

type dmRepo struct{ db *sql.DB }

func (r *dmRepo) CreateDM(ctx context.Context, user1ID, user2ID string) (_ *domain.Room, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	room := &domain.Room{}
	id := uuid.New().String()
	now := time.Now().UTC()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO rooms (id, name, is_dm, created_by, created_at) VALUES ($1, NULL, true, $2, $3) RETURNING id, name, is_dm, created_by, created_at`,
		id, user1ID, now,
	).Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return nil, err
	}

	insertMember := `INSERT INTO room_members (room_id, user_id, joined_at) VALUES ($1, $2, $3)`
	if _, err = tx.ExecContext(ctx, insertMember, room.ID, user1ID, now); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, insertMember, room.ID, user2ID, now); err != nil {
		return nil, err
	}

	err = tx.Commit()
	return room, err
}

func (r *dmRepo) GetByUsers(ctx context.Context, user1ID, user2ID string) (*domain.Room, error) {
	room := &domain.Room{}
	err := r.db.QueryRowContext(ctx,
		`SELECT r.id, r.name, r.is_dm, r.created_by, r.created_at FROM rooms r JOIN room_members rm1 ON rm1.room_id = r.id AND rm1.user_id = $1 JOIN room_members rm2 ON rm2.room_id = r.id AND rm2.user_id = $2 WHERE r.is_dm = true LIMIT 1`,
		user1ID, user2ID,
	).Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return room, nil
}

func (r *dmRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Room, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT r.id, r.name, r.is_dm, r.created_by, r.created_at FROM rooms r JOIN room_members rm ON rm.room_id = r.id WHERE rm.user_id = $1 AND r.is_dm = true ORDER BY r.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []*domain.Room
	for rows.Next() {
		room := &domain.Room{}
		if err := rows.Scan(&room.ID, &room.Name, &room.IsDM, &room.CreatedBy, &room.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}
	return rooms, rows.Err()
}

func (r *dmRepo) GetOtherMember(ctx context.Context, roomID, myUserID string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.username FROM users u JOIN room_members rm ON rm.user_id = u.id WHERE rm.room_id = $1 AND u.id != $2 LIMIT 1`,
		roomID, myUserID,
	).Scan(&u.ID, &u.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// messageRepo — implements MessageRepository
// ---------------------------------------------------------------------------

type messageRepo struct{ db *sql.DB }

func (r *messageRepo) Create(ctx context.Context, id, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
	msg := &domain.Message{}
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO messages (id, room_id, sender_id, sender_username, content, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, room_id, sender_id, sender_username, content, created_at`,
		id, roomID, senderID, senderUsername, content, time.Now().UTC(),
	).Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderUsername, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

const maxMessageLimit = 200

func (r *messageRepo) ListByRoom(ctx context.Context, roomID string, limit, offset int) ([]*domain.Message, error) {
	if limit <= 0 || limit > maxMessageLimit {
		limit = maxMessageLimit
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, room_id, sender_id, sender_username, content, created_at FROM messages WHERE room_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		roomID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		msg := &domain.Message{}
		if err := rows.Scan(&msg.ID, &msg.RoomID, &msg.SenderID, &msg.SenderUsername, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// Compile-time interface assertions.
var (
	_ UserRepository    = (*userRepo)(nil)
	_ RoomRepository    = (*roomRepo)(nil)
	_ DMRepository      = (*dmRepo)(nil)
	_ MessageRepository = (*messageRepo)(nil)
)
