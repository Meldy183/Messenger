#!/usr/bin/env bash
# Usage: ./scripts/load-test.sh [requests]
# Default: 50 rounds of requests against auth-service and chat-service.

AUTH="http://localhost:8080"
CHAT="http://localhost:8081"
ROUNDS=${1:-50}

echo "→ Registering test user (safe to fail if already exists)..."
curl -sf -X POST "$AUTH/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"loadtest","password":"loadtest123"}' > /dev/null 2>&1 || true

echo "→ Logging in..."
TOKEN=$(curl -sf -X POST "$AUTH/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"loadtest","password":"loadtest123"}' \
  | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

[ -z "$TOKEN" ] && echo "✗ Login failed — is docker-compose up?" && exit 1

echo "→ Creating test room..."
ROOM_ID=$(curl -sf -X POST "$CHAT/api/v1/rooms" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"load-test-room"}' \
  | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 2>/dev/null || true)

echo "→ Running $ROUNDS rounds..."

for i in $(seq 1 "$ROUNDS"); do
  # 2xx — normal traffic
  curl -sf "$CHAT/api/v1/rooms"            -H "Authorization: Bearer $TOKEN" > /dev/null &
  curl -sf "$CHAT/api/v1/users/me"         -H "Authorization: Bearer $TOKEN" > /dev/null &
  curl -sf "$AUTH/api/v1/auth/login"       -H "Content-Type: application/json" \
    -d '{"username":"loadtest","password":"loadtest123"}' > /dev/null &

  # 401 — no token (populates error_rate panel without killing services)
  curl -sf "$CHAT/api/v1/rooms"            > /dev/null 2>&1 &
  curl -sf "$CHAT/api/v1/users/me"         > /dev/null 2>&1 &

  # 400 — bad body
  curl -sf -X POST "$AUTH/api/v1/auth/register" \
    -H "Content-Type: application/json" -d '{}' > /dev/null 2>&1 &

  # room endpoint if room was created
  [ -n "$ROOM_ID" ] && \
    curl -sf "$CHAT/api/v1/rooms/$ROOM_ID/messages" \
      -H "Authorization: Bearer $TOKEN" > /dev/null &

  if (( i % 10 == 0 )); then
    wait
    echo "  $i / $ROUNDS"
  fi
done

wait
echo "✓ Done — open Grafana: http://localhost:3001"
