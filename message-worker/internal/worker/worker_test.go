package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
)

// ── persist ───────────────────────────────────────────────────────────────────

func TestPersist_NewMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	want := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO messages`).
		WithArgs("msg-1", "room-1", "user-1", "alice", "hello world").
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(want))

	w := &Worker{db: db, log: zap.NewNop()}
	msg := KafkaMessage{
		ID:             "msg-1",
		RoomID:         "room-1",
		SenderID:       "user-1",
		SenderUsername: "alice",
		Content:        "hello world",
	}

	createdAt, inserted, err := w.persist(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inserted {
		t.Fatal("expected inserted=true for a new message")
	}
	if !createdAt.Equal(want) {
		t.Fatalf("expected createdAt=%v, got %v", want, createdAt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled sqlmock expectations: %v", err)
	}
}

func TestPersist_DuplicateMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// ON CONFLICT DO NOTHING → no rows returned → sql.ErrNoRows
	mock.ExpectQuery(`INSERT INTO messages`).
		WithArgs("msg-dup", "room-1", "user-1", "alice", "hello again").
		WillReturnRows(sqlmock.NewRows([]string{"created_at"})) // empty result set

	w := &Worker{db: db, log: zap.NewNop()}
	msg := KafkaMessage{
		ID:             "msg-dup",
		RoomID:         "room-1",
		SenderID:       "user-1",
		SenderUsername: "alice",
		Content:        "hello again",
	}

	_, inserted, err := w.persist(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error for duplicate: %v", err)
	}
	if inserted {
		t.Fatal("expected inserted=false for a duplicate message")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unfulfilled sqlmock expectations: %v", err)
	}
}

// ── broadcast ─────────────────────────────────────────────────────────────────

func TestBroadcast_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/broadcast" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	w := &Worker{chatServiceURL: srv.URL, log: zap.NewNop()}
	msg := KafkaMessage{
		ID:             "msg-1",
		RoomID:         "room-1",
		SenderID:       "user-1",
		SenderUsername: "alice",
		Content:        "hello",
	}

	if err := w.broadcast(msg, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBroadcast_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	w := &Worker{chatServiceURL: srv.URL, log: zap.NewNop()}
	msg := KafkaMessage{
		ID:             "msg-2",
		RoomID:         "room-1",
		SenderID:       "user-1",
		SenderUsername: "alice",
		Content:        "will fail",
	}

	if err := w.broadcast(msg, time.Now()); err == nil {
		t.Fatal("expected an error for 500 response, got nil")
	}
}
