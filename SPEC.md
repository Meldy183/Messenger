# Messenger — Project Specification (MVP_0)

## Origin & MVP Anchor

This specification is pinned to the following project pitch (MVP_0 definition):

> "Combining B and C. The idea is to have separate Kafka topics for each chat room, so users
> can join specific rooms and only receive messages from those, plus message filtering within
> rooms for things like direct messages visible only to certain users. One of my teammates is
> interested in frontend, so we'd also like to build a simple UI for it — essentially making it
> into a basic messenger app to tie everything together."

Every feature below must trace back to this anchor. Anything not derivable from it is out of scope for MVP_0.

---

## Team

| Person | Responsibility |
|--------|----------------|
| Fyodor | Kafka integration, message pipeline, overall architecture |
| Backend #2 | Kubernetes manifests, local cluster setup, service deployment |
| Frontend | React UI, WebSocket client, room/account management views |

---

## Tech Stack

| Layer | Technology |
|-------|------------|
| Backend language | Go |
| Message broker | Apache Kafka |
| Database | PostgreSQL |
| Auth | JWT (HS256) |
| Real-time transport | WebSocket |
| Container orchestration | Kubernetes (local — minikube or kind) |
| Frontend | React |

---

## Core Concepts

### Room
A named, persistent chat channel. Every room maps **1-to-1** to a Kafka topic
(`room.<room_uuid>`). Any registered user can create or join any room. All members of a
room receive all messages posted to it.

### Direct Message (DM)
A private room with exactly two members. Implemented as a regular room with:
- `is_dm = true` flag
- membership locked to the two participants (no one else can join)
- Kafka topic: `room.<room_uuid>` (same pattern, no special casing in the broker)

### Message
A piece of text sent by a user to a room. Assigned a UUID at creation time. Persisted to
PostgreSQL. Delivered via Kafka → WebSocket.

---

## Functional Requirements

### FR-1: Authentication

| ID | Requirement |
|----|-------------|
| FR-1.1 | User can register with a unique username and password |
| FR-1.2 | User can log in and receive a signed JWT |
| FR-1.3 | All endpoints except `/register` and `/login` require a valid JWT |
| FR-1.4 | Passwords are stored hashed (bcrypt) |

### FR-2: Users

| ID | Requirement |
|----|-------------|
| FR-2.1 | Authenticated user can retrieve their own profile |
| FR-2.2 | Authenticated user can list all registered users (needed for DM initiation) |

### FR-3: Rooms

| ID | Requirement |
|----|-------------|
| FR-3.1 | Authenticated user can create a room with a unique name |
| FR-3.2 | Room creation provisions a Kafka topic `room.<room_uuid>` |
| FR-3.3 | Authenticated user can list all public rooms |
| FR-3.4 | Authenticated user can join any public room (open joining) |
| FR-3.5 | Authenticated user can leave a room they are a member of |
| FR-3.6 | Authenticated user can list rooms they have joined |

### FR-4: Direct Messages

| ID | Requirement |
|----|-------------|
| FR-4.1 | Authenticated user can initiate a DM with another user |
| FR-4.2 | If a DM between the two users already exists, return the existing room |
| FR-4.3 | DM rooms are not listed in the public room list |
| FR-4.4 | Only the two participants can send/receive messages in a DM room |
| FR-4.5 | Authenticated user can list their active DM conversations |

### FR-5: Messaging

| ID | Requirement |
|----|-------------|
| FR-5.1 | Authenticated user connects to the server via WebSocket |
| FR-5.2 | User can send a message to any room they are a member of |
| FR-5.3 | User receives messages in real-time for all rooms they are currently connected to |
| FR-5.4 | Messages are produced to Kafka topic `room.<room_uuid>` |
| FR-5.5 | A Kafka consumer reads messages, persists them to PostgreSQL, then broadcasts to connected WebSocket clients in that room |
| FR-5.6 | Kafka offset is committed **after** successful PostgreSQL write |
| FR-5.7 | Authenticated user can fetch message history for any room they are a member of (paginated, newest-first) |

### FR-6: Kafka Topic Lifecycle

| ID | Requirement |
|----|-------------|
| FR-6.1 | A Kafka topic is created when a room is created |
| FR-6.2 | Topic name format: `room.<room_uuid>` |
| FR-6.3 | Each topic has 1 partition and replication factor 1 (sufficient for local dev) |

---

## Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-1 | All business logic lives in the backend; the frontend is display-only |
| NFR-2 | Messages are deduplicated in PostgreSQL using the UUID primary key (`ON CONFLICT DO NOTHING`) to handle Kafka at-least-once redelivery |
| NFR-3 | Kafka delivery guarantee: **at-least-once** (producer `acks=all`, consumer commits after DB write) |
| NFR-4 | JWT expiry: 24 hours |
| NFR-5 | The full system runs on a local Kubernetes cluster (minikube or kind) |
| NFR-6 | Kubernetes resources are defined as plain YAML manifests (no Helm) |

---

## Explicitly Out of Scope for MVP_0

- Message editing or deletion
- File / image uploads
- Message reactions or emoji
- Read receipts
- Online presence / typing indicators
- Push notifications
- Message search
- Room moderation (kick, ban, roles)
- OAuth / social login
- Pagination cursor (offset-based is sufficient)

---

## Data Model (logical)

```
users
  id          UUID PK
  username    TEXT UNIQUE NOT NULL
  password    TEXT NOT NULL          -- bcrypt hash
  created_at  TIMESTAMPTZ

rooms
  id          UUID PK
  name        TEXT UNIQUE            -- NULL for DMs
  is_dm       BOOL NOT NULL DEFAULT false
  created_by  UUID FK → users.id
  created_at  TIMESTAMPTZ

room_members
  room_id     UUID FK → rooms.id
  user_id     UUID FK → users.id
  joined_at   TIMESTAMPTZ
  PRIMARY KEY (room_id, user_id)

messages
  id          UUID PK                -- assigned by producer, dedup key
  room_id     UUID FK → rooms.id
  sender_id   UUID FK → users.id
  content     TEXT NOT NULL
  created_at  TIMESTAMPTZ
```

---

## Message Flow (detailed)

```
[React Client]
    │  WebSocket send: { room_id, content }
    ▼
[API Server — Go]
    │  1. Validate JWT, check sender is member of room
    │  2. Assign UUID as message_id
    │  3. Produce to Kafka topic "room.<room_id>"
    │     Producer config: acks=all, idempotent=true
    ▼
[Kafka — topic: room.<room_id>]
    ▼
[Consumer — Go]
    │  4. Read message from Kafka
    │  5. INSERT INTO messages ... ON CONFLICT (id) DO NOTHING
    │  6. If inserted: broadcast to WebSocket hub → all connected members of room
    │  7. Commit Kafka offset
    ▼
[React Clients in that room]
    receive message via WebSocket
```

**Crash recovery:** If the consumer crashes between step 5 and step 7, Kafka redelivers the
message. The `ON CONFLICT DO NOTHING` on the UUID prevents a duplicate row. The WebSocket
broadcast may fire twice in this scenario (known trade-off, acceptable for MVP_0).

---

## API Surface (REST + WebSocket)

### Auth
```
POST /api/v1/auth/register    { username, password }
POST /api/v1/auth/login       { username, password } → { token }
```

### Users
```
GET  /api/v1/users            → [ { id, username } ]
GET  /api/v1/users/me         → { id, username, created_at }
```

### Rooms
```
POST /api/v1/rooms            { name } → room
GET  /api/v1/rooms            → [ room ]          (public rooms only)
GET  /api/v1/rooms/me         → [ room ]          (joined rooms)
POST /api/v1/rooms/:id/join
POST /api/v1/rooms/:id/leave
```

### Direct Messages
```
POST /api/v1/dms              { user_id } → room  (creates or returns existing DM)
GET  /api/v1/dms              → [ room ]
```

### Messages
```
GET  /api/v1/rooms/:id/messages?limit=50&offset=0  → [ message ]
```

### WebSocket
```
WS   /ws                      (auth via ?token=<jwt> query param)

Client → Server frames:
  { "type": "send_message", "room_id": "<uuid>", "content": "..." }

Server → Client frames:
  { "type": "new_message", "message": { id, room_id, sender_id, content, created_at } }
  { "type": "error", "message": "..." }
```

---

## Kubernetes Deployment (local)

Services to deploy:
- `postgres` — StatefulSet + PersistentVolumeClaim
- `kafka` + `zookeeper` (or KRaft mode) — StatefulSet
- `messenger-api` — Deployment (the Go backend)
- `messenger-frontend` — Deployment (React, served via nginx)

Each service exposed via a Kubernetes `Service`. API + frontend exposed via `Ingress` or
`NodePort` for local access.

---

## Delivery Guarantee — Decision Record

**Choice:** at-least-once (ALO)

**Rationale:** True exactly-once (EOS) in Kafka requires transactional producers and
consumers, which adds significant complexity. ALO with UUID-based deduplication in Postgres
achieves effectively-once storage semantics with far less code. The only observable
difference is a potential duplicate WebSocket frame to the client on consumer crash — an
acceptable trade-off for a pet project and a valid discussion point in interviews.

**How to explain in an interview:**
> "We chose ALO because EOS requires Kafka transactions which are complex to operate. Since
> we persist to Postgres with a UUID primary key, storage is idempotent regardless of
> redelivery. We accept the small risk of a duplicate push event on the client, which is
> easy to deduplicate client-side if needed."
