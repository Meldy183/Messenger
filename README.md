# Messenger

A real-time chat application built to explore Kafka-driven messaging, microservice architecture, and Kubernetes deployment. Users join public rooms or start direct messages — all delivered in real time through a Kafka pipeline.

---

## Features

- JWT authentication (register / login)
- Public chat rooms — create or join
- Direct messages — private 1-on-1 conversations
- Real-time delivery via WebSocket
- Message history in PostgreSQL
- At-least-once Kafka delivery with UUID deduplication
- Full observability: metrics (Prometheus/Grafana) + structured logs (Loki)
- Runs on docker-compose or Kubernetes

---

## Architecture

```
┌──────────────────────────────────────────────┐
│            React Frontend  :3000             │
└─────────────────┬────────────────────────────┘
                  │ HTTP + WebSocket
      ┌───────────┴───────────┐
      ▼                       ▼
┌─────────────┐      ┌────────────────────┐
│auth-service │      │   chat-service     │
│   :8080     │      │      :8081         │
│ JWT issuance│      │ rooms · DMs · WS   │
└─────────────┘      │ Kafka producer     │
                     └────────┬───────────┘
                              │ topic: room.<uuid>
                              ▼
                       ┌─────────────┐
                       │    Kafka    │
                       └──────┬──────┘
                              │
                    ┌─────────▼──────────┐
                    │  message-worker    │
                    │     :8082          │
                    │ persist · broadcast│
                    └─────────┬──────────┘
                              │
                    ┌─────────▼──────────┐
                    │    PostgreSQL      │
                    └────────────────────┘

        Observability
        ─────────────
        services ──/metrics──► Prometheus ──► Grafana :3001
        container logs ──────► Promtail ──► Loki ──► Grafana
        Prometheus ──────────► Alertmanager :9093
```

---

## Tech Stack

| Layer          | Technology                                        |
|----------------|---------------------------------------------------|
| Backend        | Go 1.22+                                          |
| Message broker | Apache Kafka                                      |
| Database       | PostgreSQL 16                                     |
| Authentication | JWT (HS256, 24 h)                                 |
| Real-time      | WebSocket (`gorilla/websocket`)                   |
| Frontend       | React 18 + TypeScript                             |
| Orchestration  | Kubernetes (minikube / kind)                      |
| Observability  | Prometheus, Grafana, Loki, Promtail, Alertmanager |

---

## Services

### `auth-service` — port 8080

Handles registration and login. Issues signed JWTs. All other services validate tokens locally — no inter-service call per request.

```
POST /api/v1/auth/register   { username, password } → UserFull
POST /api/v1/auth/login      { username, password } → { token }
GET  /health                 → 200
GET  /ready                  → 200 | 503 (pings Postgres)
GET  /metrics                → Prometheus text
```

---

### `chat-service` — port 8081

Owns rooms, users, DMs, and WebSocket connections. Produces messages to Kafka.

```
GET  /api/v1/users               → all users
GET  /api/v1/users/me            → current user

POST /api/v1/rooms               { name } → room
GET  /api/v1/rooms               → public rooms
GET  /api/v1/rooms/me            → joined rooms
POST /api/v1/rooms/{id}/join
POST /api/v1/rooms/{id}/leave
GET  /api/v1/rooms/{id}/messages → history (limit/offset)

POST /api/v1/dms                 { user_id } → DM room
GET  /api/v1/dms                 → DM list

WS   /ws?token=<jwt>             → real-time channel
GET  /health  /ready  /metrics
POST /internal/broadcast         (message-worker only)
```

WebSocket messages:

```
Client → Server:  { "type": "send_message", "room_id": "...", "content": "..." }
Server → Client:  { "type": "new_message",  "message": { id, room_id, sender_id, content, created_at } }
                  { "type": "error",         "message": "..." }
```

---

### `message-worker` — metrics on port 8082

Stateless Kafka consumer. For each message:
1. `INSERT INTO messages ... ON CONFLICT (id) DO NOTHING` — UUID dedup
2. `POST /internal/broadcast` → chat-service pushes to WS clients
3. Commit Kafka offset

No public HTTP surface. Prometheus metrics on `:8082/metrics`.

---

## Message Flow

```
User sends message
  │
  ▼ WS frame { type: send_message, room_id, content }
chat-service → validates JWT, checks membership, assigns UUID
  └─► produce to Kafka topic room.<room_id>
        │
        ▼
message-worker
  ├─ INSERT ON CONFLICT DO NOTHING
  ├─ if inserted → POST /internal/broadcast
  └─ commit offset
        │
        ▼
All room members receive { type: new_message, message: {...} }
```

Crash recovery: if the worker crashes after the Postgres write but before the offset commit, Kafka redelivers. The `ON CONFLICT` clause deduplicates silently. The broadcast may fire twice — accepted trade-off for MVP_0.

---

## Data Model

```sql
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,          -- bcrypt
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE rooms (
    id          UUID PRIMARY KEY,
    name        TEXT UNIQUE,            -- NULL for DMs
    is_dm       BOOLEAN NOT NULL DEFAULT false,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE room_members (
    room_id   UUID REFERENCES rooms(id),
    user_id   UUID REFERENCES users(id),
    joined_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE messages (
    id              UUID PRIMARY KEY,   -- assigned by chat-service; dedup key
    room_id         UUID REFERENCES rooms(id),
    sender_id       UUID REFERENCES users(id),
    sender_username TEXT NOT NULL,      -- denormalized
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ DEFAULT now()
);
```

---

## Project Structure

```
messenger/
├── auth-service/
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── domain/
│   │   ├── handler/     # HTTP handlers
│   │   ├── service/
│   │   └── repository/
│   └── Dockerfile
│
├── chat-service/
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── domain/
│   │   ├── handler/     # HTTP + WebSocket handlers
│   │   ├── hub/         # WebSocket hub
│   │   ├── kafka/       # producer
│   │   ├── repository/
│   │   └── service/
│   └── Dockerfile
│
├── message-worker/
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── config/
│   │   └── worker/      # Kafka consumer, persistence, broadcast
│   └── Dockerfile
│
├── frontend/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── contexts/
│   │   └── pages/
│   └── Dockerfile
│
├── pkg/
│   ├── logger/          # zap + context injection
│   ├── middleware/       # Auth, Logger, Metrics
│   ├── postgres/
│   └── response/         # JSON envelope helpers
│
├── db/schema.sql
│
├── monitoring/
│   ├── prometheus/        # prometheus.yml + alert-rules.yml
│   ├── alertmanager/
│   ├── loki/
│   ├── promtail/          # promtail-docker.yml + promtail-k8s.yml
│   └── grafana/provisioning/
│
├── k8s/
│   ├── namespace.yaml
│   ├── ingress.yaml
│   ├── postgres/ kafka/ auth-service/ chat-service/ message-worker/ frontend/
│   └── monitoring/        # K8s manifests for the full observability stack
│
├── scripts/load-test.sh
└── docker-compose.yml
```

---

## Quick Start (docker-compose)

```bash
docker-compose up -d
```

| Service      | URL                   |
|--------------|-----------------------|
| Frontend     | http://localhost:3000 |
| auth-service | http://localhost:8080 |
| chat-service | http://localhost:8081 |
| Grafana      | http://localhost:3001 |
| Prometheus   | http://localhost:9090 |
| Alertmanager | http://localhost:9093 |

```bash
docker-compose down -v   # stop + clean volumes
```

---

## Load Testing

```bash
./scripts/load-test.sh          # 50 rounds (default)
./scripts/load-test.sh 200      # custom count
```

Each round fires mixed parallel requests: 200s, 401s (no token), and 400s (bad body). After the run, open the **Messenger Overview** dashboard in Grafana to see request rate, error rate, latency, and message throughput.

---

## Monitoring

| Component    | Purpose                                      | Port |
|--------------|----------------------------------------------|------|
| Prometheus   | Scrapes `/metrics` from all services         | 9090 |
| Alertmanager | Alert routing                                | 9093 |
| Loki         | Log aggregation                              | 3100 |
| Promtail     | Log shipping (Docker socket / K8s DaemonSet) | —    |
| Grafana      | Dashboards + log exploration                 | 3001 |

**Alert:** `HighHTTPErrorRate` — 5xx rate > 5% over 5 min → severity: warning.

**Exposed metrics:**

| Metric                                      | Type      | Labels                        |
|---------------------------------------------|-----------|-------------------------------|
| `messenger_http_request_duration_seconds`   | histogram | service, method, path, status |
| `messenger_http_requests_total`             | counter   | service, method, path, status |
| `messenger_ws_connections_active`           | gauge     | —                             |
| `messenger_messages_processed_total`        | counter   | status (inserted / duplicate) |

**Grafana dashboard "Messenger Overview"** — 8 panels: request rate, error rate, p95/p99 latency, active WS connections, messages processed, duplicate rate, live logs.

### Kubernetes

```bash
kubectl apply -f k8s/monitoring/prometheus/
kubectl apply -f k8s/monitoring/alertmanager/
kubectl apply -f k8s/monitoring/loki/
kubectl apply -f k8s/monitoring/promtail/
kubectl apply -f k8s/monitoring/grafana/
```

Promtail requires cluster-level RBAC — already defined in `k8s/monitoring/promtail/`.

---

## Kubernetes Deploy

### Prerequisites

- minikube or kind, `kubectl`, Docker

### Build images

```bash
# If using minikube: eval $(minikube docker-env) first
docker build -t messenger/auth-service:dev    ./auth-service
docker build -t messenger/chat-service:dev    ./chat-service
docker build -t messenger/message-worker:dev  ./message-worker
docker build -t messenger/frontend:dev        ./frontend
```

### Deploy

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/postgres/ -f k8s/kafka/
kubectl wait --for=condition=ready pod -l app=postgres -n messenger --timeout=120s
kubectl wait --for=condition=ready pod -l app=kafka    -n messenger --timeout=120s
kubectl apply -f k8s/auth-service/ -f k8s/chat-service/ \
              -f k8s/message-worker/ -f k8s/frontend/
kubectl apply -f k8s/ingress.yaml
```

### Access

```bash
minikube service frontend -n messenger
# or: minikube tunnel → http://messenger.local
```

---

## Kafka Design

| Decision           | Choice                    | Reason                                      |
|--------------------|---------------------------|---------------------------------------------|
| Topic per room     | Yes                       | Per-room consumer isolation                 |
| Delivery guarantee | At-least-once             | Postgres UUID PK handles deduplication      |
| Partitions         | 1 per topic               | No ordering concerns at this scale          |
| Consumer group     | Single (`message-worker`) | All messages must be processed exactly once |

---

## Out of Scope

Message editing/deletion, file uploads, reactions, read receipts, typing indicators, push notifications, message search, room moderation, OAuth.
