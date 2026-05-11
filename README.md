# Messenger

A real-time chat application built as a pet project to explore Kafka-driven messaging, microservice architecture, and Kubernetes deployment. Users can join public chat rooms and exchange direct messages — all delivered in real time through a Kafka message pipeline.

---

## Features

- JWT authentication (register / login)
- Public chat rooms — open for anyone to create or join
- Direct messages — private 1-on-1 conversations
- Real-time message delivery via WebSocket
- Message history persisted in PostgreSQL
- At-least-once Kafka delivery with UUID-based deduplication
- One Kafka topic per room (`room.<uuid>`)
- Fully containerized and deployed on a local Kubernetes cluster

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        React Frontend                           │
│   Login  │  Room List  │  Chat View  │  DM View  │  Account     │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP + WebSocket
          ┌──────────────┴──────────────┐
          │                             │
          ▼                             ▼
 ┌─────────────────┐          ┌──────────────────┐
 │  auth-service   │          │   chat-service   │
 │                 │          │                  │
 │  POST /register │          │  REST API        │
 │  POST /login    │          │  (rooms, users,  │
 │  Issues JWT     │          │   DMs, history)  │
 └─────────────────┘          │                  │
          │                   │  WebSocket hub   │
          │ shared JWT secret │  (online users)  │
          │  via K8s Secret   │                  │
          └──────────────────►│  Kafka producer  │
                              └────────┬─────────┘
                                       │ produces to
                                       │ topic: room.<uuid>
                                       ▼
                              ┌─────────────────┐
                              │      Kafka      │
                              │                 │
                              │ topic per room  │
                              └────────┬────────┘
                                       │ consumes from
                                       ▼
                              ┌─────────────────────────────┐
                              │      message-worker         │
                              │                             │
                              │  1. Write to PostgreSQL     │
                              │     ON CONFLICT DO NOTHING  │
                              │  2. POST /internal/broadcast│
                              │     → chat-service          │
                              │  3. Commit Kafka offset     │
                              └─────────────┬───────────────┘
                                            │
                              ┌─────────────┴───────────────┐
                              │         PostgreSQL          │
                              │  users, rooms, members,     │
                              │  messages                   │
                              └─────────────────────────────┘
```

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend language | Go 1.22+ |
| Message broker | Apache Kafka |
| Database | PostgreSQL 16 |
| Authentication | JWT (HS256, 24h expiry) |
| Real-time transport | WebSocket (`gorilla/websocket`) |
| Container orchestration | Kubernetes (minikube / kind) |
| Frontend | React 18 + TypeScript |
| Observability | Prometheus · Grafana · Loki · Promtail · Alertmanager |

---

## Services

### `auth-service`
Handles user registration and login. Issues signed JWTs. Stores users in PostgreSQL.
All other services validate tokens **locally** using the shared JWT secret — no inter-service call on each request.

**Endpoints:**
```
POST /api/v1/auth/register   { username, password }
POST /api/v1/auth/login      { username, password } → { token }
GET  /health                 → 200 (liveness)
GET  /ready                  → 200 | 503 (readiness — pings Postgres)
GET  /metrics                → Prometheus text format
```

---

### `chat-service`
The core service. Owns room/user/DM management, holds WebSocket connections for all online users, and produces messages to Kafka.

**REST endpoints:**
```
GET  /api/v1/users              → list all users
GET  /api/v1/users/me           → current user profile

POST /api/v1/rooms              { name } → room
GET  /api/v1/rooms              → public rooms
GET  /api/v1/rooms/me           → joined rooms
POST /api/v1/rooms/:id/join
POST /api/v1/rooms/:id/leave

POST /api/v1/dms                { user_id } → room (creates or returns existing)
GET  /api/v1/dms                → DM conversations

GET  /api/v1/rooms/:id/messages?limit=50&offset=0 → message history

GET  /health                    → 200 (liveness)
GET  /ready                     → 200 | 503 (readiness — pings Postgres)
GET  /metrics                   → Prometheus text format
```

**WebSocket:**
```
WS /ws?token=<jwt>

Client → Server:
  { "type": "send_message", "room_id": "<uuid>", "content": "..." }

Server → Client:
  { "type": "new_message", "message": { id, room_id, sender_id, content, created_at } }
  { "type": "error",       "message": "..." }
```

**Internal endpoint (called by message-worker only):**
```
POST /internal/broadcast   { room_id, message } → broadcasts to WS clients in that room
```

---

### `message-worker`
Stateless Kafka consumer. For each message consumed:
1. Inserts into PostgreSQL (`ON CONFLICT (id) DO NOTHING` — deduplication)
2. Calls `chat-service /internal/broadcast`
3. Commits Kafka offset

Exposes `/metrics` on port **8082** for Prometheus scraping. No other external HTTP surface.

Runs as a background Deployment in K8s; periodic heartbeat file (`/tmp/worker-heartbeat`) can be used for liveness checks.

---

## Message Flow

```
User types a message and presses send
  │
  ▼ WebSocket frame { type: send_message, room_id, content }
chat-service
  ├── validates JWT
  ├── checks sender is a member of the room
  ├── assigns UUID as message_id
  └── produces to Kafka topic "room.<room_id>"  [acks=all]
  │
  ▼ Kafka topic: room.<room_id>
message-worker
  ├── reads message
  ├── INSERT INTO messages ... ON CONFLICT (id) DO NOTHING
  ├── if inserted → POST /internal/broadcast to chat-service
  └── commit Kafka offset
  │
  ▼ chat-service WebSocket hub
All connected members of the room receive:
  { type: new_message, message: { ... } }
```

**Crash recovery:** if `message-worker` crashes after the Postgres write but before the offset commit, Kafka redelivers the message. The `ON CONFLICT` clause silently ignores the duplicate. The broadcast may fire twice (known trade-off — acceptable for MVP_0).

---

## Data Model

```sql
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,  -- bcrypt
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE rooms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT UNIQUE,    -- NULL for DMs
    is_dm       BOOLEAN NOT NULL DEFAULT false,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE room_members (
    room_id     UUID REFERENCES rooms(id),
    user_id     UUID REFERENCES users(id),
    joined_at   TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (room_id, user_id)
);

CREATE TABLE messages (
    id              UUID PRIMARY KEY,  -- assigned by chat-service, dedup key
    room_id         UUID REFERENCES rooms(id),
    sender_id       UUID REFERENCES users(id),
    sender_username TEXT NOT NULL,     -- denormalized for read performance
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
│   │   ├── service/     # business logic
│   │   └── repository/  # Postgres queries
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
│   │   ├── api/         # REST client
│   │   ├── components/
│   │   ├── contexts/    # auth context
│   │   └── pages/       # Login, Register, Rooms, Chat, DMs
│   └── Dockerfile
│
├── pkg/
│   ├── logger/          # zap setup + context injection
│   ├── middleware/       # Auth, Logger, Metrics HTTP middleware
│   ├── postgres/         # database connection helper
│   └── response/         # JSON envelope helpers
│
├── db/
│   └── schema.sql
│
├── monitoring/
│   ├── prometheus/        # prometheus.yml + alert-rules.yml
│   ├── alertmanager/      # alertmanager.yml
│   ├── loki/              # loki-config.yml
│   ├── promtail/          # promtail-docker.yml + promtail-k8s.yml
│   └── grafana/
│       └── provisioning/  # datasources + auto-provisioned dashboard
│
├── k8s/
│   ├── namespace.yaml
│   ├── ingress.yaml
│   ├── postgres/
│   ├── kafka/
│   ├── auth-service/
│   ├── chat-service/
│   ├── message-worker/
│   ├── frontend/
│   └── monitoring/        # K8s manifests for the full observability stack
│
├── scripts/
│   └── load-test.sh       # automated load test against docker-compose
│
├── docker-compose.yml
├── SPEC.md
└── README.md
```

---

## Quick Start (docker-compose)

The fastest way to run the full stack locally — no cluster required.

```bash
docker-compose up -d
```

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| auth-service | http://localhost:8080 |
| chat-service | http://localhost:8081 |
| message-worker metrics | http://localhost:8082/metrics |
| Grafana | http://localhost:3001 |
| Prometheus | http://localhost:9090 |
| Alertmanager | http://localhost:9093 |

To stop and clean up volumes:

```bash
docker-compose down -v
```

---

## Load Testing

`scripts/load-test.sh` generates mixed traffic against the running stack (default: 50 rounds).

```bash
chmod +x scripts/load-test.sh
./scripts/load-test.sh           # 50 rounds
./scripts/load-test.sh 200       # custom round count
```

Each round fires parallel requests covering:
- Authenticated `GET /rooms`, `GET /users/me` (expect 200)
- `POST /auth/login` (expect 200)
- Unauthenticated `GET /rooms` and `GET /users/me` (expect 401)
- `POST /auth/register` with an empty body (expect 400)

After the script completes, open Grafana at http://localhost:3001 and look at the **Messenger Overview** dashboard to see request rate, error rate, latency, and message throughput panels populated with real data.

---

## Local Development (Kubernetes)

### Prerequisites
- [minikube](https://minikube.sigs.k8s.io/) or [kind](https://kind.sigs.k8s.io/)
- `kubectl`
- Docker

### Start the cluster

```bash
minikube start
minikube addons enable ingress
# or
kind create cluster --name messenger
```

> If you use minikube and want to build images directly into the cluster Docker daemon,
> run `eval $(minikube docker-env)` before `docker build`.

### Build container images

```bash
docker build -t messenger/auth-service:dev ./auth-service
docker build -t messenger/chat-service:dev ./chat-service
docker build -t messenger/message-worker:dev ./message-worker
docker build -t messenger/frontend:dev ./frontend
```

If you are **not** using `eval $(minikube docker-env)`, load the images into minikube explicitly:

```bash
minikube image load messenger/auth-service:dev
minikube image load messenger/chat-service:dev
minikube image load messenger/message-worker:dev
minikube image load messenger/frontend:dev
```

### Deploy

```bash
# Create namespace
kubectl apply -f k8s/namespace.yaml

# Infrastructure
kubectl apply -f k8s/postgres/
kubectl apply -f k8s/kafka/

# Wait for infra to be ready
kubectl wait --for=condition=ready pod -l app=postgres -n messenger --timeout=120s
kubectl wait --for=condition=ready pod -l app=kafka   -n messenger --timeout=120s

# Services
kubectl apply -f k8s/auth-service/
kubectl apply -f k8s/chat-service/
kubectl apply -f k8s/message-worker/
kubectl apply -f k8s/frontend/

# Wait for app pods to be ready
kubectl wait --for=condition=ready pod -l app=auth-service   -n messenger --timeout=120s
kubectl wait --for=condition=ready pod -l app=chat-service   -n messenger --timeout=120s
kubectl wait --for=condition=ready pod -l app=message-worker -n messenger --timeout=120s
kubectl wait --for=condition=ready pod -l app=frontend       -n messenger --timeout=120s

# Ingress
kubectl apply -f k8s/ingress.yaml
```

### Access the app

```bash
minikube service frontend -n messenger
# or via ingress:
minikube tunnel  # then visit http://messenger.local
```

If you use ingress with a custom local hostname such as `messenger.local`, add it to `/etc/hosts` if needed.

### Health checks and resource requests

Each application Deployment should define:

- `resources.requests` and `resources.limits`
- `readinessProbe` to signal when the container is ready to receive traffic
- `livenessProbe` to restart the container if it becomes unhealthy

Example snippet for a stateless service:

```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"

readinessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 15
  periodSeconds: 20
```

These settings help avoid routing traffic to a container that has started but is not yet ready, and they make local debugging in minikube much easier.

### Troubleshooting

Use the following commands to inspect the cluster if something does not start correctly:

```bash
kubectl get pods -n messenger
kubectl describe pod <pod-name> -n messenger
kubectl logs <pod-name> -n messenger
kubectl get svc -n messenger
kubectl get ingress -n messenger
kubectl exec -it <pod-name> -n messenger -- sh
```

What they are useful for:

- `kubectl get pods` — check whether pods are running, pending, or crashing
- `kubectl describe pod` — inspect events such as failed mounts, image pull errors, failed probes, or scheduling issues
- `kubectl logs` — read application logs from the container
- `kubectl get svc` — verify that Services were created and expose the expected ports
- `kubectl get ingress` — confirm that ingress rules were created correctly
- `kubectl exec` — enter a running container to inspect environment variables, DNS, and network connectivity from inside the cluster

---

## Monitoring

### What was added

| Component | Purpose | Port |
|-----------|---------|------|
| Prometheus | Metrics scraping | 9090 |
| Alertmanager | Alert routing | 9093 |
| Loki | Log aggregation | 3100 |
| Promtail | Log shipping (Docker socket for docker-compose; DaemonSet for K8s) | — |
| Grafana | Dashboards + log exploration | 3001 (host) |

Pre-configured alert: **HighHTTPErrorRate** — HTTP 5xx rate > 5% over a 5-minute window → severity: warning.

### Metrics exposed by the app

| Metric | Type | Labels | Service |
|--------|------|--------|---------|
| `messenger_http_request_duration_seconds` | histogram | service, method, path, status | all HTTP services |
| `messenger_http_requests_total` | counter | service, method, path, status | all HTTP services |
| `messenger_ws_connections_active` | gauge | — | chat-service |
| `messenger_messages_processed_total` | counter | status (inserted or duplicate) | message-worker |

All HTTP services expose a `/metrics` endpoint. `message-worker` exposes it on port 8082.

### Running with docker-compose

```bash
docker-compose up -d
```

Access:
- Grafana: http://localhost:3001 (anonymous admin, no login required)
- Prometheus: http://localhost:9090
- Alertmanager: http://localhost:9093

### Running on Kubernetes

```bash
kubectl apply -f k8s/monitoring/prometheus/
kubectl apply -f k8s/monitoring/alertmanager/
kubectl apply -f k8s/monitoring/loki/
kubectl apply -f k8s/monitoring/promtail/
kubectl apply -f k8s/monitoring/grafana/
```

Note: Promtail requires cluster-level RBAC (ClusterRole/ClusterRoleBinding already defined in `k8s/monitoring/promtail/`).

### Dashboard

The "Messenger Overview" dashboard is auto-provisioned in Grafana with 8 panels:

- Request rate and HTTP error rate by service
- p95/p99 latency by service
- Active WebSocket connections
- Messages processed rate (inserted vs duplicate)
- Duplicate message rate %
- Live application logs (from Loki)

### File layout

```
monitoring/
├── prometheus/        # prometheus.yml + alert-rules.yml
├── alertmanager/      # alertmanager.yml
├── loki/              # loki-config.yml
├── promtail/          # promtail-docker.yml (compose) + promtail-k8s.yml (K8s)
└── grafana/
    └── provisioning/  # datasources + auto-provisioned dashboard

k8s/monitoring/        # K8s manifests for all monitoring components
```

---

## 3-Day Roadmap

### Day 1 — Foundation

| Who | Tasks |
|-----|-------|
| **Fyodor** | Repository structure, Go module setup per service, shared JWT validation middleware, `auth-service` complete (register, login, Postgres), Kafka admin client utility (topic provisioning on room create) |
| **Backend #2** | minikube cluster up, Postgres StatefulSet + PVC + Service, Kafka + Zookeeper StatefulSet, `namespace.yaml`, verify both are reachable inside the cluster |
| **Frontend** | Vite + React + TypeScript scaffold, React Router setup, Login and Register pages wired to auth-service REST, JWT stored in `localStorage`, protected route wrapper |

**End of Day 1 checkpoint:** user can register and log in. Postgres and Kafka running in K8s.

---

### Day 2 — Core Messaging

| Who | Tasks |
|-----|-------|
| **Fyodor** | `chat-service`: room CRUD, membership (join/leave), WebSocket hub, Kafka producer on message send, `/internal/broadcast` endpoint. `message-worker`: Kafka consumer loop, Postgres write with dedup, broadcast call to chat-service |
| **Backend #2** | K8s Deployments + Services for `auth-service`, `chat-service`, `message-worker`. K8s Secret for JWT secret shared across services. ConfigMaps for DB/Kafka connection strings. Internal DNS wiring (worker → chat-service) |
| **Frontend** | Room list page, room chat view, WebSocket client hook (`useWebSocket`), send message input, real-time message rendering, message history fetch on room open |

**End of Day 2 checkpoint:** user can create a room, join it, send messages, and see them appear in real time in another browser tab.

---

### Day 3 — DMs, Integration, Polish

| Who | Tasks |
|-----|-------|
| **Fyodor** | DM endpoint (create/list), ensure DM rooms are excluded from public listing, end-to-end smoke test of full message flow, bug fixes |
| **Backend #2** | Ingress setup, `minikube tunnel` or NodePort for external access, full cluster smoke test (all services healthy), fix any pod networking issues |
| **Frontend** | DM list + DM chat view, account page (display username), error states (disconnected WS, failed send), basic responsive layout |

**End of Day 3 checkpoint:** full demo-ready application running in K8s. Register two users, open two browsers, join the same room and exchange messages in real time, start a DM.

---

## Kafka Design Decisions

| Decision | Choice | Reason |
|----------|--------|--------|
| Topic per room | Yes | Core requirement from MVP_0; enables per-room consumer isolation |
| Delivery guarantee | At-least-once | Simpler than exactly-once transactions; storage dedup via Postgres UUID PK achieves effectively-once semantics |
| Partition count | 1 per topic | No ordering concerns at this scale; keeps setup simple |
| Consumer group | One group for `message-worker` | All messages must be processed; no competing consumers needed |

---

## Out of Scope (MVP_0)

- Message editing / deletion
- File or image uploads
- Reactions / emoji
- Read receipts
- Typing indicators / online presence
- Push notifications
- Message search
- Room moderation (kick, ban, roles)
- OAuth / social login
