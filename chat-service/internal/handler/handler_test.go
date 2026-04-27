package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/fyodor/messenger/chat-service/internal/domain"
	"github.com/fyodor/messenger/chat-service/internal/hub"
	"github.com/fyodor/messenger/chat-service/internal/service"
	"github.com/fyodor/messenger/pkg/middleware"
)

// ── mock services ──────────────────────────────────────────────────────────────

type mockUserSvc struct {
	listFn    func(ctx context.Context) ([]*domain.User, error)
	getByIDFn func(ctx context.Context, id string) (*domain.User, error)
}

func (m *mockUserSvc) List(ctx context.Context) ([]*domain.User, error) {
	return m.listFn(ctx)
}
func (m *mockUserSvc) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return m.getByIDFn(ctx, id)
}

type mockRoomSvc struct {
	createFn     func(ctx context.Context, name, createdBy string) (*domain.Room, error)
	joinFn       func(ctx context.Context, roomID, userID string) error
	leaveFn      func(ctx context.Context, roomID, userID string) error
	listPublicFn func(ctx context.Context) ([]*domain.Room, error)
	listJoinedFn func(ctx context.Context, userID string) ([]*domain.Room, error)
	isMemberFn   func(ctx context.Context, roomID, userID string) (bool, error)
}

func (m *mockRoomSvc) Create(ctx context.Context, name, createdBy string) (*domain.Room, error) {
	return m.createFn(ctx, name, createdBy)
}
func (m *mockRoomSvc) Join(ctx context.Context, roomID, userID string) error {
	return m.joinFn(ctx, roomID, userID)
}
func (m *mockRoomSvc) Leave(ctx context.Context, roomID, userID string) error {
	return m.leaveFn(ctx, roomID, userID)
}
func (m *mockRoomSvc) ListPublic(ctx context.Context) ([]*domain.Room, error) {
	return m.listPublicFn(ctx)
}
func (m *mockRoomSvc) ListJoined(ctx context.Context, userID string) ([]*domain.Room, error) {
	return m.listJoinedFn(ctx, userID)
}
func (m *mockRoomSvc) IsMember(ctx context.Context, roomID, userID string) (bool, error) {
	return m.isMemberFn(ctx, roomID, userID)
}

type mockDMSvc struct {
	createOrGetFn func(ctx context.Context, requesterID, targetUserID string) (*domain.DmRoom, error)
	listFn        func(ctx context.Context, userID string) ([]*domain.DmRoom, error)
}

func (m *mockDMSvc) CreateOrGet(ctx context.Context, requesterID, targetUserID string) (*domain.DmRoom, error) {
	return m.createOrGetFn(ctx, requesterID, targetUserID)
}
func (m *mockDMSvc) List(ctx context.Context, userID string) ([]*domain.DmRoom, error) {
	return m.listFn(ctx, userID)
}

type mockMsgSvc struct {
	sendFn    func(ctx context.Context, roomID, senderID, senderUsername, content string) (*domain.Message, error)
	historyFn func(ctx context.Context, roomID, userID string, limit, offset int) ([]*domain.Message, error)
}

func (m *mockMsgSvc) Send(ctx context.Context, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
	return m.sendFn(ctx, roomID, senderID, senderUsername, content)
}
func (m *mockMsgSvc) History(ctx context.Context, roomID, userID string, limit, offset int) ([]*domain.Message, error) {
	return m.historyFn(ctx, roomID, userID, limit, offset)
}

// ── helpers ────────────────────────────────────────────────────────────────────

const testSecret = "test-secret"

// Well-formed UUIDs for use in path params.
const (
	testRoomUUID = "11111111-1111-1111-1111-111111111111"
	testUserUUID = "22222222-2222-2222-2222-222222222222"
	testDMUUID   = "33333333-3333-3333-3333-333333333333"
)

// signedToken creates a HS256 JWT with the given subject and the shared test secret.
func signedToken(subject string) string {
	claims := jwt.RegisteredClaims{
		Subject: subject,
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		panic(err)
	}
	return tok
}

// withUserID injects a userID into the context the same way middleware.Auth does.
func withUserID(userID string, next http.Handler) http.Handler {
	return middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}))
}

// newAuthedRequest returns a request with a valid Bearer token for userID.
func newAuthedRequest(method, target, userID string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set("Authorization", "Bearer "+signedToken(userID))
	return req
}

// newHandler builds a Handler with all mock services and a real hub.
func newHandler(
	userSvc service.UserService,
	roomSvc service.RoomService,
	dmSvc service.DMService,
	msgSvc service.MessageService,
	h *hub.Hub,
) *Handler {
	return New(userSvc, roomSvc, dmSvc, msgSvc, h, nil, testSecret, "")
}

func defaultUserSvc() *mockUserSvc {
	return &mockUserSvc{
		listFn: func(ctx context.Context) ([]*domain.User, error) {
			return []*domain.User{{ID: testUserUUID, Username: "alice"}}, nil
		},
		getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
			return &domain.User{ID: id, Username: "alice"}, nil
		},
	}
}

func defaultRoomSvc() *mockRoomSvc {
	name := "general"
	return &mockRoomSvc{
		createFn: func(ctx context.Context, n, createdBy string) (*domain.Room, error) {
			return &domain.Room{ID: testRoomUUID, Name: &name, CreatedBy: createdBy, CreatedAt: time.Now()}, nil
		},
		joinFn:  func(ctx context.Context, roomID, userID string) error { return nil },
		leaveFn: func(ctx context.Context, roomID, userID string) error { return nil },
		listPublicFn: func(ctx context.Context) ([]*domain.Room, error) {
			return []*domain.Room{{ID: testRoomUUID, Name: &name}}, nil
		},
		listJoinedFn: func(ctx context.Context, userID string) ([]*domain.Room, error) {
			return []*domain.Room{{ID: testRoomUUID, Name: &name}}, nil
		},
		isMemberFn: func(ctx context.Context, roomID, userID string) (bool, error) { return true, nil },
	}
}

func defaultDMSvc() *mockDMSvc {
	return &mockDMSvc{
		createOrGetFn: func(ctx context.Context, r, t string) (*domain.DmRoom, error) {
			return &domain.DmRoom{
				Room:      &domain.Room{ID: testDMUUID},
				OtherUser: &domain.User{ID: t, Username: "bob"},
			}, nil
		},
		listFn: func(ctx context.Context, userID string) ([]*domain.DmRoom, error) {
			return []*domain.DmRoom{}, nil
		},
	}
}

func defaultMsgSvc() *mockMsgSvc {
	return &mockMsgSvc{
		sendFn: func(ctx context.Context, roomID, senderID, senderUsername, content string) (*domain.Message, error) {
			return &domain.Message{ID: "m1", RoomID: roomID, SenderID: senderID, Content: content}, nil
		},
		historyFn: func(ctx context.Context, roomID, userID string, limit, offset int) ([]*domain.Message, error) {
			return []*domain.Message{{ID: "m1", RoomID: roomID, SenderID: userID, Content: "hello"}}, nil
		},
	}
}

// chiCtx wraps a handler so that chi URL params are available via chi.URLParam.
func chiCtx(paramKey, paramVal string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add(paramKey, paramVal)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		next.ServeHTTP(w, r)
	})
}

// ── ListUsers ─────────────────────────────────────────────────────────────────

func TestListUsers(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/users", testUserUUID, nil)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.ListUsers)).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("service error 500", func(t *testing.T) {
		userSvc := &mockUserSvc{
			listFn: func(ctx context.Context) ([]*domain.User, error) {
				return nil, errors.New("db down")
			},
		}
		h := newHandler(userSvc, defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/users", testUserUUID, nil)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.ListUsers)).ServeHTTP(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rr.Code)
		}
	})
}

// ── Me ────────────────────────────────────────────────────────────────────────

func TestMe(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/users/me", testUserUUID, nil)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.Me)).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("not found 404", func(t *testing.T) {
		userSvc := &mockUserSvc{
			getByIDFn: func(ctx context.Context, id string) (*domain.User, error) {
				return nil, service.ErrNotFound
			},
		}
		h := newHandler(userSvc, defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/users/me", testUserUUID, nil)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.Me)).ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})
}

// ── CreateRoom ────────────────────────────────────────────────────────────────

func TestCreateRoom(t *testing.T) {
	t.Run("success 201", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"name": "general"})
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateRoom)).ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("name taken 409", func(t *testing.T) {
		roomSvc := defaultRoomSvc()
		roomSvc.createFn = func(ctx context.Context, name, createdBy string) (*domain.Room, error) {
			return nil, service.ErrRoomNameTaken
		}
		h := newHandler(defaultUserSvc(), roomSvc, defaultDMSvc(), defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"name": "taken"})
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateRoom)).ServeHTTP(rr, req)
		if rr.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rr.Code)
		}
	})

	t.Run("empty name 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"name": "   "})
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateRoom)).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})
}

// ── ListRooms ─────────────────────────────────────────────────────────────────

func TestListRooms(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/rooms", testUserUUID, nil)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.ListRooms)).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ── ListJoinedRooms ───────────────────────────────────────────────────────────

func TestListJoinedRooms(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/rooms/me", testUserUUID, nil)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.ListJoinedRooms)).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ── JoinRoom ──────────────────────────────────────────────────────────────────

func TestJoinRoom(t *testing.T) {
	t.Run("success 204", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms/"+testRoomUUID+"/join", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", testRoomUUID, withUserID(testUserUUID, http.HandlerFunc(h.JoinRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("not found 404", func(t *testing.T) {
		roomSvc := defaultRoomSvc()
		roomSvc.joinFn = func(ctx context.Context, roomID, userID string) error { return service.ErrNotFound }
		h := newHandler(defaultUserSvc(), roomSvc, defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms/"+testRoomUUID+"/join", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", testRoomUUID, withUserID(testUserUUID, http.HandlerFunc(h.JoinRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})

	t.Run("is DM 403", func(t *testing.T) {
		roomSvc := defaultRoomSvc()
		roomSvc.joinFn = func(ctx context.Context, roomID, userID string) error { return service.ErrIsDMRoom }
		h := newHandler(defaultUserSvc(), roomSvc, defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms/"+testDMUUID+"/join", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", testDMUUID, withUserID(testUserUUID, http.HandlerFunc(h.JoinRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("empty room id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms//join", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", "", withUserID(testUserUUID, http.HandlerFunc(h.JoinRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid uuid room id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms/not-a-uuid/join", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", "not-a-uuid", withUserID(testUserUUID, http.HandlerFunc(h.JoinRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ── LeaveRoom ─────────────────────────────────────────────────────────────────

func TestLeaveRoom(t *testing.T) {
	t.Run("success 204", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms/"+testRoomUUID+"/leave", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", testRoomUUID, withUserID(testUserUUID, http.HandlerFunc(h.LeaveRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("empty room id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms//leave", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", "", withUserID(testUserUUID, http.HandlerFunc(h.LeaveRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid uuid room id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodPost, "/api/v1/rooms/not-a-uuid/leave", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", "not-a-uuid", withUserID(testUserUUID, http.HandlerFunc(h.LeaveRoom))).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ── ListMessages ──────────────────────────────────────────────────────────────

func TestListMessages(t *testing.T) {
	t.Run("success 200", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/rooms/"+testRoomUUID+"/messages", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", testRoomUUID, withUserID(testUserUUID, http.HandlerFunc(h.ListMessages))).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("not member 403", func(t *testing.T) {
		msgSvc := defaultMsgSvc()
		msgSvc.historyFn = func(ctx context.Context, roomID, userID string, limit, offset int) ([]*domain.Message, error) {
			return nil, service.ErrForbidden
		}
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), msgSvc, hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/rooms/"+testRoomUUID+"/messages", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", testRoomUUID, withUserID(testUserUUID, http.HandlerFunc(h.ListMessages))).ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("empty room id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/rooms//messages", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", "", withUserID(testUserUUID, http.HandlerFunc(h.ListMessages))).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid uuid room id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		req := newAuthedRequest(http.MethodGet, "/api/v1/rooms/not-a-uuid/messages", testUserUUID, nil)
		rr := httptest.NewRecorder()
		chiCtx("id", "not-a-uuid", withUserID(testUserUUID, http.HandlerFunc(h.ListMessages))).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ── CreateOrGetDM ─────────────────────────────────────────────────────────────

func TestCreateOrGetDM(t *testing.T) {
	t.Run("success 201", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"user_id": testUserUUID})
		req := newAuthedRequest(http.MethodPost, "/api/v1/dms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateOrGetDM)).ServeHTTP(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("empty user_id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"user_id": ""})
		req := newAuthedRequest(http.MethodPost, "/api/v1/dms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateOrGetDM)).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid uuid user_id 400", func(t *testing.T) {
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"user_id": "not-a-uuid"})
		req := newAuthedRequest(http.MethodPost, "/api/v1/dms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateOrGetDM)).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("self DM 400", func(t *testing.T) {
		dmSvc := defaultDMSvc()
		dmSvc.createOrGetFn = func(ctx context.Context, r, t string) (*domain.DmRoom, error) {
			return nil, service.ErrSelfDM
		}
		h := newHandler(defaultUserSvc(), defaultRoomSvc(), dmSvc, defaultMsgSvc(), hub.New())
		body, _ := json.Marshal(map[string]string{"user_id": testDMUUID})
		req := newAuthedRequest(http.MethodPost, "/api/v1/dms", testUserUUID, body)
		rr := httptest.NewRecorder()
		withUserID(testUserUUID, http.HandlerFunc(h.CreateOrGetDM)).ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

// ── InternalBroadcast ─────────────────────────────────────────────────────────

func TestInternalBroadcast(t *testing.T) {
	t.Run("success 204 — hub.Broadcast called", func(t *testing.T) {
		h := hub.New()

		client := &hub.Client{
			UserID:   testUserUUID,
			Username: "alice",
			Send:     make(chan []byte, 8),
		}
		h.Subscribe(testRoomUUID, client)

		handler := newHandler(defaultUserSvc(), defaultRoomSvc(), defaultDMSvc(), defaultMsgSvc(), h)

		payload := map[string]interface{}{
			"room_id": testRoomUUID,
			"message": map[string]interface{}{
				"id":              "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				"room_id":         testRoomUUID,
				"sender_id":       testUserUUID,
				"sender_username": "alice",
				"content":         "hello",
				"created_at":      time.Now().Format(time.RFC3339),
			},
		}
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/internal/broadcast", bytes.NewReader(body))
		rr := httptest.NewRecorder()

		handler.InternalBroadcast(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
		}

		select {
		case msg := <-client.Send:
			if !strings.Contains(string(msg), "new_message") {
				t.Fatalf("unexpected frame: %s", msg)
			}
		default:
			t.Fatal("expected client to receive a broadcast message but channel was empty")
		}
	})
}
