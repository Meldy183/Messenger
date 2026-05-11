//go:build integration

package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *apiError       `json:"error"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type userResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type roomResponse struct {
	ID string `json:"id"`
}

type dmResponse struct {
	Room struct {
		ID string `json:"id"`
	} `json:"room"`
	OtherUser struct {
		ID string `json:"id"`
	} `json:"other_user"`
}

func baseURLFromEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return strings.TrimRight(v, "/")
}

func request(t *testing.T, client *http.Client, method, url string, body any, token string) (int, envelope) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("build request %s %s: %v", method, url, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if len(rawBody) == 0 {
		return resp.StatusCode, envelope{}
	}

	var env envelope
	if err := json.Unmarshal(rawBody, &env); err != nil {
		t.Fatalf("decode response (status=%d): %v; body=%s", resp.StatusCode, err, string(rawBody))
	}
	return resp.StatusCode, env
}

func decodeData[T any](t *testing.T, env envelope) T {
	t.Helper()
	var out T
	if len(env.Data) == 0 {
		t.Fatal("response has empty data field")
	}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("decode data field: %v", err)
	}
	return out
}

func requireStatus(t *testing.T, got, want int, env envelope) {
	t.Helper()
	if got != want {
		if env.Error != nil {
			t.Fatalf("expected status %d, got %d (%d: %s)", want, got, env.Error.Code, env.Error.Message)
		}
		t.Fatalf("expected status %d, got %d", want, got)
	}
}

func roomExists(rooms []roomResponse, roomID string) bool {
	for _, r := range rooms {
		if r.ID == roomID {
			return true
		}
	}
	return false
}

func dmExists(dms []dmResponse, otherUserID string) bool {
	for _, dm := range dms {
		if dm.OtherUser.ID == otherUserID {
			return true
		}
	}
	return false
}

func TestHTTPSmoke(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	authBase := baseURLFromEnv("AUTH_BASE_URL", "http://localhost:8080")
	chatBase := baseURLFromEnv("CHAT_BASE_URL", "http://localhost:8081")

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user1 := "ci-user-1-" + suffix
	user2 := "ci-user-2-" + suffix
	password := "ci-pass-123"
	roomName := "ci-room-" + suffix

	status, env := request(t, client, http.MethodPost, authBase+"/api/v1/auth/register", map[string]string{
		"username": user1,
		"password": password,
	}, "")
	requireStatus(t, status, http.StatusCreated, env)
	register1 := decodeData[userResponse](t, env)

	status, env = request(t, client, http.MethodPost, authBase+"/api/v1/auth/register", map[string]string{
		"username": user2,
		"password": password,
	}, "")
	requireStatus(t, status, http.StatusCreated, env)
	register2 := decodeData[userResponse](t, env)

	status, env = request(t, client, http.MethodPost, authBase+"/api/v1/auth/login", map[string]string{
		"username": user1,
		"password": password,
	}, "")
	requireStatus(t, status, http.StatusOK, env)
	login := decodeData[loginResponse](t, env)
	if login.Token == "" {
		t.Fatal("expected non-empty JWT token")
	}

	status, env = request(t, client, http.MethodGet, chatBase+"/api/v1/users", nil, login.Token)
	requireStatus(t, status, http.StatusOK, env)

	status, env = request(t, client, http.MethodGet, chatBase+"/api/v1/users/me", nil, login.Token)
	requireStatus(t, status, http.StatusOK, env)
	me := decodeData[userResponse](t, env)
	if me.ID != register1.ID {
		t.Fatalf("expected /users/me id %s, got %s", register1.ID, me.ID)
	}

	status, env = request(t, client, http.MethodPost, chatBase+"/api/v1/rooms", map[string]string{
		"name": roomName,
	}, login.Token)
	requireStatus(t, status, http.StatusCreated, env)
	room := decodeData[roomResponse](t, env)
	if room.ID == "" {
		t.Fatal("expected non-empty room id")
	}

	status, env = request(t, client, http.MethodGet, chatBase+"/api/v1/rooms", nil, login.Token)
	requireStatus(t, status, http.StatusOK, env)
	publicRooms := decodeData[[]roomResponse](t, env)
	if !roomExists(publicRooms, room.ID) {
		t.Fatalf("created room %s not found in /rooms", room.ID)
	}

	status, env = request(t, client, http.MethodGet, chatBase+"/api/v1/rooms/me", nil, login.Token)
	requireStatus(t, status, http.StatusOK, env)
	joinedRooms := decodeData[[]roomResponse](t, env)
	if !roomExists(joinedRooms, room.ID) {
		t.Fatalf("created room %s not found in /rooms/me", room.ID)
	}

	status, env = request(t, client, http.MethodPost, chatBase+"/api/v1/rooms/"+room.ID+"/join", nil, login.Token)
	requireStatus(t, status, http.StatusNoContent, env)

	status, env = request(t, client, http.MethodGet, chatBase+"/api/v1/rooms/"+room.ID+"/messages?limit=50&offset=0", nil, login.Token)
	requireStatus(t, status, http.StatusOK, env)

	status, env = request(t, client, http.MethodPost, chatBase+"/api/v1/dms", map[string]string{
		"user_id": register2.ID,
	}, login.Token)
	requireStatus(t, status, http.StatusCreated, env)
	dm := decodeData[dmResponse](t, env)
	if dm.Room.ID == "" {
		t.Fatal("expected non-empty DM room id")
	}
	if dm.OtherUser.ID != register2.ID {
		t.Fatalf("expected DM other user %s, got %s", register2.ID, dm.OtherUser.ID)
	}

	status, env = request(t, client, http.MethodGet, chatBase+"/api/v1/dms", nil, login.Token)
	requireStatus(t, status, http.StatusOK, env)
	dms := decodeData[[]dmResponse](t, env)
	if !dmExists(dms, register2.ID) {
		t.Fatalf("expected DM with user %s in /dms", register2.ID)
	}
}
