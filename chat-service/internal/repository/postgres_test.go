package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func newTestRepo(t *testing.T) (*PostgresRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewPostgresRepository(db), mock
}

// ---------------------------------------------------------------------------
// UserRepository
// ---------------------------------------------------------------------------

func TestUserList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"id", "username"}).
			AddRow("user-1", "alice").
			AddRow("user-2", "bob")
		mock.ExpectQuery(`SELECT id, username FROM users ORDER BY username`).
			WillReturnRows(rows)

		users, err := repo.AsUserRepo().List(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(users) != 2 {
			t.Fatalf("expected 2 users, got %d", len(users))
		}
		if users[0].Username != "alice" {
			t.Errorf("expected alice, got %s", users[0].Username)
		}
		if users[1].Username != "bob" {
			t.Errorf("expected bob, got %s", users[1].Username)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestUserGetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"id", "username", "created_at"}).
			AddRow("user-1", "alice", time.Now())
		mock.ExpectQuery(`SELECT id, username, created_at FROM users WHERE id = \$1`).
			WithArgs("user-1").
			WillReturnRows(rows)

		user, err := repo.AsUserRepo().GetByID(ctx, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != "user-1" {
			t.Errorf("expected user-1, got %s", user.ID)
		}
		if user.Username != "alice" {
			t.Errorf("expected alice, got %s", user.Username)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		mock.ExpectQuery(`SELECT id, username, created_at FROM users WHERE id = \$1`).
			WithArgs("missing").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.AsUserRepo().GetByID(ctx, "missing")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// RoomRepository
// ---------------------------------------------------------------------------

func TestRoomCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		roomName := "general"
		now := time.Now().UTC()

		mock.ExpectBegin()
		roomRows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("room-1", &roomName, false, "user-1", now)
		mock.ExpectQuery(`INSERT INTO rooms \(id, name, is_dm, created_by, created_at\) VALUES \(\$1, \$2, false, \$3, \$4\) RETURNING id, name, is_dm, created_by, created_at`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "user-1", sqlmock.AnyArg()).
			WillReturnRows(roomRows)
		mock.ExpectExec(`INSERT INTO room_members \(room_id, user_id, joined_at\) VALUES \(\$1, \$2, \$3\)`).
			WithArgs("room-1", "user-1", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		room, err := repo.AsRoomRepo().Create(ctx, "general", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if room.Name == nil || *room.Name != "general" {
			t.Errorf("expected name 'general', got %v", room.Name)
		}
		if room.IsDM {
			t.Errorf("expected IsDM=false")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestRoomCreate_NameTaken(t *testing.T) {
	repo, mock := newTestRepo(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO rooms \(id, name, is_dm, created_by, created_at\) VALUES \(\$1, \$2, false, \$3, \$4\) RETURNING id, name, is_dm, created_by, created_at`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), "user-1", sqlmock.AnyArg()).
		WillReturnError(&pq.Error{Code: "23505"})
	mock.ExpectRollback()

	_, err := repo.AsRoomRepo().Create(ctx, "general", "user-1")
	if err != ErrRoomNameTaken {
		t.Errorf("expected ErrRoomNameTaken, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestRoomGetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		roomName := "general"
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("room-1", &roomName, false, "user-1", now)
		mock.ExpectQuery(`SELECT id, name, is_dm, created_by, created_at FROM rooms WHERE id = \$1`).
			WithArgs("room-1").
			WillReturnRows(rows)

		room, err := repo.AsRoomRepo().GetByID(ctx, "room-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if room.ID != "room-1" {
			t.Errorf("expected room-1, got %s", room.ID)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		mock.ExpectQuery(`SELECT id, name, is_dm, created_by, created_at FROM rooms WHERE id = \$1`).
			WithArgs("missing").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.AsRoomRepo().GetByID(ctx, "missing")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestRoomListPublic(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		name1 := "general"
		name2 := "random"
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("room-1", &name1, false, "user-1", now).
			AddRow("room-2", &name2, false, "user-2", now)
		mock.ExpectQuery(`SELECT id, name, is_dm, created_by, created_at FROM rooms WHERE is_dm = false ORDER BY created_at DESC`).
			WillReturnRows(rows)

		rooms, err := repo.AsRoomRepo().ListPublic(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rooms) != 2 {
			t.Fatalf("expected 2 rooms, got %d", len(rooms))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestRoomListByMember(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		roomName := "general"
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("room-1", &roomName, false, "user-1", now)
		mock.ExpectQuery(`SELECT r\.id, r\.name, r\.is_dm, r\.created_by, r\.created_at FROM rooms r JOIN room_members rm ON rm\.room_id = r\.id WHERE rm\.user_id = \$1 AND r\.is_dm = false ORDER BY r\.created_at DESC`).
			WithArgs("user-1").
			WillReturnRows(rows)

		rooms, err := repo.AsRoomRepo().ListByMember(ctx, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rooms) != 1 {
			t.Fatalf("expected 1 room, got %d", len(rooms))
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestRoomAddMember(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		mock.ExpectExec(`INSERT INTO room_members \(room_id, user_id, joined_at\) VALUES \(\$1, \$2, \$3\) ON CONFLICT DO NOTHING`).
			WithArgs("room-1", "user-2", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.AsRoomRepo().AddMember(ctx, "room-1", "user-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestRoomRemoveMember(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		mock.ExpectExec(`DELETE FROM room_members WHERE room_id = \$1 AND user_id = \$2`).
			WithArgs("room-1", "user-2").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.AsRoomRepo().RemoveMember(ctx, "room-1", "user-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestRoomIsMember(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
		mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM room_members WHERE room_id = \$1 AND user_id = \$2\)`).
			WithArgs("room-1", "user-1").
			WillReturnRows(rows)

		isMember, err := repo.AsRoomRepo().IsMember(ctx, "room-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isMember {
			t.Errorf("expected true, got false")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("false", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)
		mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM room_members WHERE room_id = \$1 AND user_id = \$2\)`).
			WithArgs("room-1", "user-99").
			WillReturnRows(rows)

		isMember, err := repo.AsRoomRepo().IsMember(ctx, "room-1", "user-99")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isMember {
			t.Errorf("expected false, got true")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// DMRepository
// ---------------------------------------------------------------------------

func TestCreateDM(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		now := time.Now().UTC()

		mock.ExpectBegin()
		dmRows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("dm-room-1", nil, true, "user-1", now)
		mock.ExpectQuery(`INSERT INTO rooms \(id, name, is_dm, created_by, created_at\) VALUES \(\$1, NULL, true, \$2, \$3\) RETURNING id, name, is_dm, created_by, created_at`).
			WithArgs(sqlmock.AnyArg(), "user-1", sqlmock.AnyArg()).
			WillReturnRows(dmRows)
		mock.ExpectExec(`INSERT INTO room_members \(room_id, user_id, joined_at\) VALUES \(\$1, \$2, \$3\)`).
			WithArgs("dm-room-1", "user-1", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`INSERT INTO room_members \(room_id, user_id, joined_at\) VALUES \(\$1, \$2, \$3\)`).
			WithArgs("dm-room-1", "user-2", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		room, err := repo.AsDMRepo().CreateDM(ctx, "user-1", "user-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !room.IsDM {
			t.Errorf("expected IsDM=true")
		}
		if room.Name != nil {
			t.Errorf("expected nil name for DM, got %v", room.Name)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestDMGetByUsers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("dm-room-1", nil, true, "user-1", now)
		mock.ExpectQuery(`SELECT r\.id, r\.name, r\.is_dm, r\.created_by, r\.created_at FROM rooms r JOIN room_members rm1 ON rm1\.room_id = r\.id AND rm1\.user_id = \$1 JOIN room_members rm2 ON rm2\.room_id = r\.id AND rm2\.user_id = \$2 WHERE r\.is_dm = true LIMIT 1`).
			WithArgs("user-1", "user-2").
			WillReturnRows(rows)

		room, err := repo.AsDMRepo().GetByUsers(ctx, "user-1", "user-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if room.ID != "dm-room-1" {
			t.Errorf("expected dm-room-1, got %s", room.ID)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		mock.ExpectQuery(`SELECT r\.id, r\.name, r\.is_dm, r\.created_by, r\.created_at FROM rooms r JOIN room_members rm1 ON rm1\.room_id = r\.id AND rm1\.user_id = \$1 JOIN room_members rm2 ON rm2\.room_id = r\.id AND rm2\.user_id = \$2 WHERE r\.is_dm = true LIMIT 1`).
			WithArgs("user-1", "user-99").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.AsDMRepo().GetByUsers(ctx, "user-1", "user-99")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestDMListByUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "name", "is_dm", "created_by", "created_at"}).
			AddRow("dm-room-1", nil, true, "user-1", now)
		mock.ExpectQuery(`SELECT r\.id, r\.name, r\.is_dm, r\.created_by, r\.created_at FROM rooms r JOIN room_members rm ON rm\.room_id = r\.id WHERE rm\.user_id = \$1 AND r\.is_dm = true ORDER BY r\.created_at DESC`).
			WithArgs("user-1").
			WillReturnRows(rows)

		rooms, err := repo.AsDMRepo().ListByUser(ctx, "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rooms) != 1 {
			t.Fatalf("expected 1 room, got %d", len(rooms))
		}
		if !rooms[0].IsDM {
			t.Errorf("expected IsDM=true")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestDMGetOtherMember(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		rows := sqlmock.NewRows([]string{"id", "username"}).
			AddRow("user-2", "bob")
		mock.ExpectQuery(`SELECT u\.id, u\.username FROM users u JOIN room_members rm ON rm\.user_id = u\.id WHERE rm\.room_id = \$1 AND u\.id != \$2 LIMIT 1`).
			WithArgs("dm-room-1", "user-1").
			WillReturnRows(rows)

		user, err := repo.AsDMRepo().GetOtherMember(ctx, "dm-room-1", "user-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Username != "bob" {
			t.Errorf("expected bob, got %s", user.Username)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		mock.ExpectQuery(`SELECT u\.id, u\.username FROM users u JOIN room_members rm ON rm\.user_id = u\.id WHERE rm\.room_id = \$1 AND u\.id != \$2 LIMIT 1`).
			WithArgs("dm-room-1", "user-1").
			WillReturnError(sql.ErrNoRows)

		_, err := repo.AsDMRepo().GetOtherMember(ctx, "dm-room-1", "user-1")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// MessageRepository
// ---------------------------------------------------------------------------

func TestCreateMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "room_id", "sender_id", "sender_username", "content", "created_at"}).
			AddRow("msg-1", "room-1", "user-1", "alice", "hello world", now)
		mock.ExpectQuery(`INSERT INTO messages \(id, room_id, sender_id, sender_username, content, created_at\) VALUES \(\$1, \$2, \$3, \$4, \$5, \$6\) RETURNING id, room_id, sender_id, sender_username, content, created_at`).
			WithArgs("msg-1", "room-1", "user-1", "alice", "hello world", sqlmock.AnyArg()).
			WillReturnRows(rows)

		msg, err := repo.AsMessageRepo().Create(ctx, "msg-1", "room-1", "user-1", "alice", "hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg.ID != "msg-1" {
			t.Errorf("expected msg-1, got %s", msg.ID)
		}
		if msg.RoomID != "room-1" {
			t.Errorf("expected room-1, got %s", msg.RoomID)
		}
		if msg.SenderID != "user-1" {
			t.Errorf("expected user-1, got %s", msg.SenderID)
		}
		if msg.SenderUsername != "alice" {
			t.Errorf("expected alice, got %s", msg.SenderUsername)
		}
		if msg.Content != "hello world" {
			t.Errorf("expected 'hello world', got %s", msg.Content)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}

func TestMessageListByRoom(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo, mock := newTestRepo(t)
		ctx := context.Background()

		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{"id", "room_id", "sender_id", "sender_username", "content", "created_at"}).
			AddRow("msg-2", "room-1", "user-2", "bob", "hi", now).
			AddRow("msg-1", "room-1", "user-1", "alice", "hello", now.Add(-time.Minute))
		mock.ExpectQuery(`SELECT id, room_id, sender_id, sender_username, content, created_at FROM messages WHERE room_id = \$1 ORDER BY created_at DESC LIMIT \$2 OFFSET \$3`).
			WithArgs("room-1", 10, 0).
			WillReturnRows(rows)

		messages, err := repo.AsMessageRepo().ListByRoom(ctx, "room-1", 10, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(messages) != 2 {
			t.Fatalf("expected 2 messages, got %d", len(messages))
		}
		if messages[0].ID != "msg-2" {
			t.Errorf("expected msg-2 first, got %s", messages[0].ID)
		}
		if messages[1].ID != "msg-1" {
			t.Errorf("expected msg-1 second, got %s", messages[1].ID)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled expectations: %v", err)
		}
	})
}
