package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fyodor/messenger/auth-service/internal/domain"
	"github.com/fyodor/messenger/auth-service/internal/service"
)

// mockService implements service.Service for testing.
type mockService struct {
	registerFn func(ctx context.Context, username, password string) (*domain.User, error)
	loginFn    func(ctx context.Context, username, password string) (string, error)
}

func (m *mockService) Register(ctx context.Context, username, password string) (*domain.User, error) {
	return m.registerFn(ctx, username, password)
}

func (m *mockService) Login(ctx context.Context, username, password string) (string, error) {
	return m.loginFn(ctx, username, password)
}

// decodeBody decodes JSON from r into v, failing the test on error.
func decodeBody(t *testing.T, body io.Reader, v any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// responseEnvelope mirrors the shape written by pkg/response helpers.
type responseEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestRegister_Success(t *testing.T) {
	t.Run("returns 201 and username in data", func(t *testing.T) {
		mock := &mockService{
			registerFn: func(_ context.Context, username, _ string) (*domain.User, error) {
				return &domain.User{
					ID:        "user-1",
					Username:  username,
					CreatedAt: time.Now(),
				}, nil
			},
		}

		h := New(mock)
		body := strings.NewReader(`{"username":"fyodor","password":"secret"}`)
		req := httptest.NewRequest(http.MethodPost, "/register", body)
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rec.Code)
		}

		var env responseEnvelope
		decodeBody(t, rec.Body, &env)

		var userData struct {
			Username string `json:"username"`
		}
		if err := json.Unmarshal(env.Data, &userData); err != nil {
			t.Fatalf("unmarshal data field: %v", err)
		}

		if userData.Username != "fyodor" {
			t.Errorf("expected data.username %q, got %q", "fyodor", userData.Username)
		}
	})
}

func TestRegister_MissingFields(t *testing.T) {
	t.Run("returns 400 when username is empty", func(t *testing.T) {
		// registerFn should never be called; set a sentinel to catch it.
		mock := &mockService{
			registerFn: func(_ context.Context, _, _ string) (*domain.User, error) {
				t.Fatal("Register should not be called when fields are missing")
				return nil, nil
			},
		}

		h := New(mock)
		body := strings.NewReader(`{"username":"","password":"secret"}`)
		req := httptest.NewRequest(http.MethodPost, "/register", body)
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rec.Code)
		}
	})
}

func TestRegister_UsernameTaken(t *testing.T) {
	t.Run("returns 409 when username is already taken", func(t *testing.T) {
		mock := &mockService{
			registerFn: func(_ context.Context, _, _ string) (*domain.User, error) {
				return nil, service.ErrUsernameTaken
			},
		}

		h := New(mock)
		body := strings.NewReader(`{"username":"fyodor","password":"secret"}`)
		req := httptest.NewRequest(http.MethodPost, "/register", body)
		rec := httptest.NewRecorder()

		h.Register(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rec.Code)
		}
	})
}

func TestLogin_Success(t *testing.T) {
	t.Run("returns 200 and token in data", func(t *testing.T) {
		mock := &mockService{
			loginFn: func(_ context.Context, _, _ string) (string, error) {
				return "mytoken", nil
			},
		}

		h := New(mock)
		body := strings.NewReader(`{"username":"fyodor","password":"secret"}`)
		req := httptest.NewRequest(http.MethodPost, "/login", body)
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}

		var env responseEnvelope
		decodeBody(t, rec.Body, &env)

		var tokenData struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(env.Data, &tokenData); err != nil {
			t.Fatalf("unmarshal data field: %v", err)
		}

		if tokenData.Token != "mytoken" {
			t.Errorf("expected data.token %q, got %q", "mytoken", tokenData.Token)
		}
	})
}

func TestLogin_InvalidCredentials(t *testing.T) {
	t.Run("returns 401 for invalid credentials", func(t *testing.T) {
		mock := &mockService{
			loginFn: func(_ context.Context, _, _ string) (string, error) {
				return "", service.ErrInvalidCredentials
			},
		}

		h := New(mock)
		body := strings.NewReader(`{"username":"fyodor","password":"wrong"}`)
		req := httptest.NewRequest(http.MethodPost, "/login", body)
		rec := httptest.NewRecorder()

		h.Login(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})
}
