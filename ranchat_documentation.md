# Ranchat Backend — Technical Documentation

> **Version:** 1.0.0  
> **Language:** Go 1.26  
> **Last Updated:** May 2026

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Tech Stack](#2-tech-stack)
3. [Project Structure](#3-project-structure)
4. [Getting Started](#4-getting-started)
5. [Environment Variables](#5-environment-variables)
6. [Architecture](#6-architecture)
7. [API Reference](#7-api-reference)
8. [Data Models](#8-data-models)
9. [Authentication](#9-authentication)
10. [Presence System](#10-presence-system)
11. [Error Handling](#11-error-handling)
12. [Database & Indexes](#12-database--indexes)
13. [Performance](#13-performance)
14. [Roadmap](#14-roadmap)

---

## 1. Project Overview

Ranchat is an **anonymous chat application** backend. Users sign up and log in using only a **device ID** — no email, no phone number, no password. The system issues a JWT token so the user can call protected endpoints.

The backend currently handles:
- Anonymous account creation and login
- User profile management (view and update)
- Realtime presence tracking (online status, searching, typing indicators)

---

## 2. Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Language | **Go 1.26** | Extremely fast, low memory, great concurrency |
| HTTP Framework | **Gin v1.12** | Fastest Go HTTP router, great middleware support |
| Database | **MongoDB** | Flexible document store, great for JSON-shaped data |
| MongoDB Driver | **go.mongodb.org/mongo-driver v1.17** | Official driver, connection pooling built-in |
| Authentication | **JWT (HS256)** via `golang-jwt/jwt v5` | Stateless auth — no session DB needed |
| Config | **godotenv v1.5** | Load `.env` file in development |
| Hot Reload (dev) | **air** | Auto-restart server on file change |
| Presence Storage | **Go `sync.RWMutex` + `map`** | In-memory, nanosecond-speed — no DB writes for realtime state |

---

## 3. Project Structure

```
ranchat_backend/
│
├── main.go                     ← Entry point. Boots DB, starts server.
│
├── .env                        ← Local environment variables (never commit to git)
├── go.mod                      ← Go module definition and dependencies
├── go.sum                      ← Dependency checksums (auto-generated)
│
├── config/
│   └── db.go                   ← MongoDB connection, connection pool, index creation
│
├── middleware/
│   └── auth.go                 ← JWT validation middleware for protected routes
│
├── routes/
│   └── routes.go               ← Maps URL paths to handler functions
│
├── controller/
│   ├── user_controller.go      ← HTTP handlers for user endpoints
│   └── presence_controller.go  ← HTTP handlers for presence endpoints
│
├── service/
│   ├── user_service.go         ← Business logic for user operations
│   └── presence_service.go     ← Business logic for presence operations
│
├── repository/
│   ├── user_repository.go      ← MongoDB reads/writes for the users collection
│   └── presence_repository.go  ← In-memory (RAM) reads/writes for presence
│
├── models/
│   ├── user.go                 ← User struct (maps to MongoDB document)
│   ├── presence.go             ← Presence struct (lives in RAM)
│   └── requests.go             ← Request body structs + AuthResponse
│
└── utils/
    ├── response.go             ← Helpers: SendSuccess(), SendError()
    ├── time.go                 ← Helpers: FormatTime(), FormatTimePtr()
    └── jwt.go                  ← Helpers: GenerateToken(), ParseToken()
```

**Rule of thumb for where code lives:**
- **Controller** — only reads the HTTP request and writes the HTTP response. Zero business logic.
- **Service** — all business logic. Calls repository. Returns `(data, httpStatusCode, error)`.
- **Repository** — only talks to storage (MongoDB or RAM). No logic.

---

## 4. Getting Started

### Prerequisites
- Go 1.21+ installed
- MongoDB running locally (or a MongoDB Atlas connection string)
- `air` for hot reload (optional but recommended)

```bash
# Install air (hot reload tool)
go install github.com/air-verse/air@latest
```

### Run in Development

```bash
# 1. Clone the repo
git clone <repo-url>
cd ranchat_backend

# 2. Create your .env file
cp .env.example .env
# Edit .env with your MONGO_URI and JWT_SECRET

# 3. Install dependencies
go mod tidy

# 4. Run with hot reload
air

# OR run without hot reload
go run main.go
```

Server starts on `http://localhost:8080`.

### Verify the server is running

```bash
curl http://localhost:8080/health
# → { "status": "ok", "service": "ranchat-backend" }
```

---

## 5. Environment Variables

All configuration is via environment variables. In development, put them in `.env`.

| Variable | Required | Default | Description |
|---|---|---|---|
| `MONGO_URI` | No | `mongodb://localhost:27017` | MongoDB connection string |
| `DB_NAME` | No | `ranchat` | MongoDB database name |
| `PORT` | No | `8080` | HTTP server port |
| `GIN_MODE` | No | `debug` | Set to `release` in production (less verbose logs) |
| `JWT_SECRET` | **Yes** | — | Secret key for signing JWTs. **Must be 32+ random chars in production.** |
| `JWT_EXPIRY_HOURS` | No | `720` (30 days) | How long a JWT is valid |

### Example `.env`

```env
MONGO_URI=mongodb://localhost:27017
DB_NAME=ranchat
PORT=8080
GIN_MODE=debug

JWT_SECRET=ranchat-super-secret-change-in-prod-32chars!
JWT_EXPIRY_HOURS=720
```

> WARNING: Never commit `.env` to git. Add it to `.gitignore`.

---

## 6. Architecture

### Layer Diagram

```
Flutter Client
      |
      |  REST / HTTPS
      v
+---------------------------------------------+
|         Gin HTTP Server                     |
|                                             |
|  Middlewares (run on every request)         |
|    - gin.Recovery() -- catch panics         |
|    - gin.Logger()   -- log requests         |
|    - AuthRequired() -- verify JWT           |
|                                             |
|  Controllers (HTTP in/out)                  |
|    - Parse request body                     |
|    - Call service                           |
|    - Write JSON response                    |
+------------------+--------------------------+
                   |
                   v
+---------------------------------------------+
|           Service Layer                     |
|  - Business logic                           |
|  - Input validation                         |
|  - Calls repositories                       |
|  - Returns (data, status, error)            |
+----------+----------------------------------+
           |
     +-----+------+
     v            v
+---------+  +----------------------------+
| MongoDB |  |   Go In-Memory Store       |
| "users" |  |  map[userID]*Presence      |
|         |  |  + sync.RWMutex            |
| Stable  |  |                            |
| profile |  | Fast-changing realtime     |
| data    |  | state (online, typing)     |
+---------+  +----------------------------+
```

### Why Two Storage Systems?

Presence data (online/offline, searching, typing) changes **constantly** — every user sends a heartbeat every 20 seconds. At 1,000 users, that is ~50 writes/second just for heartbeats.

Writing that to MongoDB would be expensive and slow. Instead, presence lives in a Go memory map — updates take **~100 nanoseconds** vs ~5 milliseconds for a MongoDB write.

If the server restarts, presence resets — which is correct behavior. Realtime state should not persist across restarts.

---

## 7. API Reference

### Base URL
```
http://localhost:8080/api/v1
```

### Authentication
All endpoints except `POST /auth/anonymous` require a JWT in the header:
```
Authorization: Bearer <token>
```

### Response Envelope
Every response — success or error — uses the same wrapper:

```json
{
  "success": true,
  "message": "Human readable message",
  "data": { }
}
```

```json
{
  "success": false,
  "message": "What went wrong"
}
```

---

### POST /api/v1/auth/anonymous

**Register or log in with a device ID.**  
If the device has no account, a new one is created (HTTP 201). If the device already has an account, it logs in and returns the existing user (HTTP 200).

**Auth required:** No

**Request Body:**
```json
{
  "fullName":  "Mayank",
  "gender":    "male",
  "age":       22,
  "about":     "Just vibing",
  "interests": ["music", "coding"],
  "deviceId":  "unique-device-id-abc123"
}
```

| Field | Type | Required | Validation |
|---|---|---|---|
| `fullName` | string | Yes | min 1 char |
| `gender` | string | Yes | `male`, `female`, or `other` |
| `age` | int | Yes | 13 to 100 |
| `about` | string | No | — |
| `interests` | string[] | No | — |
| `deviceId` | string | Yes | must be unique per device |

**Response (201 — new user, 200 — returning user):**
```json
{
  "success": true,
  "message": "Account created",
  "data": {
    "token":     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expiresAt": "2026-06-06T00:00:00Z",
    "user": {
      "_id":         "6638a1b2c3d4e5f678901234",
      "fullName":    "Mayank",
      "gender":      "male",
      "age":         22,
      "about":       "Just vibing",
      "interests":   ["music", "coding"],
      "deviceId":    "unique-device-id-abc123",
      "lastSeen":    "2026-05-07T00:00:00Z",
      "premiumTill": null,
      "createdAt":   "2026-05-07T00:00:00Z",
      "updatedAt":   "2026-05-07T00:00:00Z"
    },
    "presence": {
      "userId":         "6638a1b2c3d4e5f678901234",
      "deviceId":       "unique-device-id-abc123",
      "lastActiveAt":   "2026-05-07T00:00:00Z",
      "isSearching":    false,
      "isMatched":      false,
      "currentMatchId": null,
      "activeChatId":   null,
      "typingTo":       null,
      "updatedAt":      "2026-05-07T00:00:00Z"
    }
  }
}
```

**curl:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/anonymous \
  -H "Content-Type: application/json" \
  -d '{
    "fullName": "Mayank",
    "gender": "male",
    "age": 22,
    "about": "Just vibing",
    "interests": ["music", "coding"],
    "deviceId": "device-abc-123"
  }'
```

---

### GET /api/v1/users/me

**Get your own profile.**

**Auth required:** Yes

**Response (200):**
```json
{
  "success": true,
  "message": "User fetched",
  "data": {
    "_id":         "6638a1b2c3d4e5f678901234",
    "fullName":    "Mayank",
    "gender":      "male",
    "age":         22,
    "about":       "Just vibing",
    "interests":   ["music", "coding"],
    "deviceId":    "device-abc-123",
    "lastSeen":    "2026-05-07T00:00:00Z",
    "premiumTill": null,
    "createdAt":   "2026-05-07T00:00:00Z",
    "updatedAt":   "2026-05-07T00:00:00Z"
  }
}
```

**curl:**
```bash
curl http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

### PATCH /api/v1/users/me

**Update your own profile. Only send the fields you want to change.**

**Auth required:** Yes

**Request Body** (all fields optional — only sent fields are updated):
```json
{
  "fullName":  "Mayank V2",
  "age":       23,
  "about":     "Still vibing",
  "interests": ["travel", "coding"],
  "gender":    "male"
}
```

| Field | Type | Validation |
|---|---|---|
| `fullName` | string | min 1 char |
| `gender` | string | `male`, `female`, or `other` |
| `age` | int | 13 to 100 |
| `about` | string | — |
| `interests` | string[] | — |

**Response (200):** Returns the full updated user object (same shape as GET /users/me).

**curl:**
```bash
curl -X PATCH http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"fullName": "Mayank V2", "age": 23}'
```

---

### GET /api/v1/users/:userId

**Get any user's profile by their ID.**

**Auth required:** Yes  
**URL Param:** `:userId` — the user's `_id` from any previous response.

**Response (200):** Same shape as GET /users/me.

**curl:**
```bash
curl http://localhost:8080/api/v1/users/6638a1b2c3d4e5f678901234 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

### POST /api/v1/presence/heartbeat

**Tell the server you are still active.**

The client must call this every **~20 seconds** while the app is in the foreground. If the server does not receive a heartbeat for 30 seconds, the user is considered **offline**.

This is an **in-memory write** — takes ~100 nanoseconds.

**Auth required:** Yes  
**Request Body:** None

**Response (200):**
```json
{
  "success": true,
  "message": "heartbeat received"
}
```

**curl:**
```bash
curl -X POST http://localhost:8080/api/v1/presence/heartbeat \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

### PATCH /api/v1/presence

**Update your own realtime presence state.**

Only send the fields you want to change. All fields are optional.

**Auth required:** Yes

**Request Body:**
```json
{
  "isSearching":  true,
  "activeChatId": "some-chat-id",
  "typingTo":     "other-user-id"
}
```

| Field | Type | Description |
|---|---|---|
| `isSearching` | bool | Set `true` when user taps "Find a stranger" |
| `activeChatId` | string or null | ID of active chat. Send `null` to clear. |
| `typingTo` | string or null | UserID being typed to. Send `null` to stop. |

**Response (200):**
```json
{
  "success": true,
  "message": "presence updated",
  "data": {
    "userId":         "6638a1b2c3d4e5f678901234",
    "deviceId":       "device-abc-123",
    "lastActiveAt":   "2026-05-07T00:00:00Z",
    "isSearching":    true,
    "isMatched":      false,
    "currentMatchId": null,
    "activeChatId":   null,
    "typingTo":       null,
    "updatedAt":      "2026-05-07T00:00:00Z"
  }
}
```

**curl — start searching:**
```bash
curl -X PATCH http://localhost:8080/api/v1/presence \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"isSearching": true}'
```

**curl — stop searching:**
```bash
curl -X PATCH http://localhost:8080/api/v1/presence \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"isSearching": false}'
```

---

### GET /api/v1/presence/:userId

**Get any user's realtime presence state.**

This is an **in-memory read** — takes ~50 nanoseconds.

**Auth required:** Yes  
**URL Param:** `:userId`

**Response (200):** Same shape as the `data` field in PATCH /presence response.

> NOTE: `isOnline` is not stored — compute it on the client:  
> `isOnline = (now - lastActiveAt) < 30 seconds`

**curl:**
```bash
curl http://localhost:8080/api/v1/presence/6638a1b2c3d4e5f678901234 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

---

## 8. Data Models

### User
Stored in MongoDB `users` collection. Holds **stable, rarely-changing profile data**.

| Field | Type | Description |
|---|---|---|
| `_id` | ObjectID | MongoDB primary key. Returned as `_id` in JSON. |
| `fullName` | string | Display name |
| `gender` | string | `"male"` / `"female"` / `"other"` |
| `age` | int | 13 to 100 |
| `about` | string | Bio / about text |
| `interests` | string[] | List of interest tags |
| `deviceId` | string | Unique device identifier. One account per device. |
| `lastSeen` | time (RFC3339) | Last login timestamp |
| `premiumTill` | time or null | Premium expiry. `null` = free user. |
| `createdAt` | time (RFC3339) | Account creation timestamp |
| `updatedAt` | time (RFC3339) | Last profile update timestamp |

---

### Presence
Stored in **Go RAM** (`map[string]*Presence`). Holds **fast-changing realtime state**.  
Resets to empty when the server restarts — this is intentional.

| Field | Type | Description |
|---|---|---|
| `userId` | ObjectID | Links back to the User |
| `deviceId` | string | Current device (updated on login) |
| `lastActiveAt` | time (RFC3339) | Last heartbeat. Use this to derive online status. |
| `isSearching` | bool | True if user is looking for a chat partner |
| `isMatched` | bool | True if user has been matched |
| `currentMatchId` | ObjectID or null | The matched user's ID |
| `activeChatId` | string or null | Chat currently open |
| `typingTo` | string or null | UserID the user is typing to |
| `updatedAt` | time (RFC3339) | Last presence change |

**How to compute isOnline on the client:**
```dart
bool isOnline = DateTime.now().difference(lastActiveAt).inSeconds < 30;
```

---

### Request Bodies

**AnonymousLoginRequest**
```
fullName  (string, required)
gender    (string, required) — "male" | "female" | "other"
age       (int,    required) — 13..100
about     (string, optional)
interests ([]string, optional)
deviceId  (string, required)
```

**UpdateUserRequest**  (all optional, only sent fields are updated)
```
fullName  (string)
gender    (string) — "male" | "female" | "other"
age       (int)    — 13..100
about     (string)
interests ([]string)
```

**UpdatePresenceRequest**  (all optional)
```
isSearching  (bool)
activeChatId (string or null)
typingTo     (string or null)
```

---

## 9. Authentication

Ranchat uses **stateless JWT authentication**. There is no session store — the token is validated on every request by re-verifying its signature.

### Flow

```
1. Client sends POST /auth/anonymous
      { fullName, gender, age, deviceId }

2. Server looks up user by deviceId in MongoDB
      - If found: update lastSeen, reuse account
      - If not found: create new User document

3. Server upserts Presence in RAM (lastActiveAt = now)

4. Server generates JWT:
      Header:  { "alg": "HS256" }
      Payload: { "userId": "...", "deviceId": "...", "exp": <unix> }
      Signed with JWT_SECRET from environment

5. Response: { token, expiresAt, user, presence }
      HTTP 201 for new user, 200 for returning user

6. Client stores the token and sends it in every request:
      Authorization: Bearer <token>

7. AuthRequired middleware (runs on every protected route):
      - Read header
      - Parse JWT, verify signature
      - Check expiry
      - Inject userID + deviceID into gin.Context
      - Call handler — or return 401 if anything is wrong
```

### Token Details
- **Algorithm:** HS256 (HMAC-SHA256)
- **Default expiry:** 720 hours (30 days), configurable via `JWT_EXPIRY_HOURS`
- **Claims:** `userId` (MongoDB ObjectID hex string), `deviceId`

### Security Notes
- Change `JWT_SECRET` to a long random string in production. Never use the default.
- Tokens cannot be revoked (stateless). For logout, the client deletes the local token.
- Future work: refresh token rotation.

---

## 10. Presence System

### Why In-Memory?

| Operation | MongoDB | Go RAM |
|---|---|---|
| Write latency | ~5 ms | ~100 ns |
| Read latency | ~3 ms | ~50 ns |
| Cost | Counts toward DB ops | Free |
| 10k heartbeats/min | Overloads cheap DB | Trivial |

### The Store (presence_repository.go)

```go
var store = &presenceStore{
    sessions: make(map[string]*Presence),  // key = userID hex string
}

type presenceStore struct {
    mu       sync.RWMutex
    sessions map[string]*Presence
}
```

### Concurrency Safety

`sync.RWMutex` allows:
- **Many goroutines to read simultaneously** — reads never block each other
- **Only one goroutine to write at a time** — writes briefly pause reads

All operations return a **copy** of the struct, not a pointer to the live map value, so callers cannot accidentally corrupt the store.

### Online Detection (Heartbeat-Based)

Instead of relying on disconnect events (unreliable on mobile), online status is **derived from the last heartbeat timestamp**:

```
isOnline = time.Since(lastActiveAt) < 30 seconds
```

If a user force-closes the app with no explicit disconnect, they are automatically considered offline after 30 seconds.

### Presence Lifecycle

```
User opens app
  -> POST /auth/anonymous
  -> UpsertPresence() in RAM (lastActiveAt = now, isSearching = false)

App is active (foreground)
  -> POST /presence/heartbeat every 20 seconds
  -> Updates lastActiveAt in RAM

User starts searching
  -> PATCH /presence { "isSearching": true }

App is closed / backgrounded
  -> No more heartbeats
  -> After 30 seconds: isOnline() returns false automatically
  -> Matchmaking ignores this user
```

---

## 11. Error Handling

### HTTP Status Codes

| Code | Meaning | When |
|---|---|---|
| `200` | OK | Request succeeded |
| `201` | Created | New user account was created |
| `400` | Bad Request | Missing fields, validation failed, bad ID format |
| `401` | Unauthorized | Missing, expired, or invalid JWT |
| `404` | Not Found | User or presence record does not exist |
| `500` | Internal Server Error | Unexpected MongoDB error or server bug |

### Error Response Shape

```json
{
  "success": false,
  "message": "authorization header is required"
}
```

The `message` field is always human-readable. On the client, you can show it directly in a snackbar or error dialog.

### Panic Safety

`gin.Recovery()` middleware wraps every handler. If any handler panics (e.g., nil pointer), the middleware catches it, logs it, and returns a `500` instead of crashing the whole server.

---

## 12. Database & Indexes

### MongoDB Collections

#### `users` collection
One document per user. Contains stable profile data.

**Indexes:**

| Index | Field | Unique | Purpose |
|---|---|---|---|
| `_id` (default) | `_id` | Yes | Primary key lookup |
| `idx_deviceId_unique` | `deviceId` | Yes | Login: find user by device. Enforces 1 account per device. |
| `idx_isSearching` | `isSearching` | No | Future matchmaking queries |

#### `presence` collection (reserved)
Indexes are created at startup but this collection is currently unused — presence lives in RAM. These indexes exist to support future database-backed presence.

| Index | Field | Unique | Purpose |
|---|---|---|---|
| `idx_presence_userId_unique` | `userId` | Yes | One presence doc per user |
| `idx_presence_isSearching` | `isSearching` | No | Find all searching users |
| `idx_presence_lastActiveAt` | `lastActiveAt` | No | Find online users |

### Connection Pool Config

```
MinPoolSize  : 5    — always-warm connections, no cold-start latency
MaxPoolSize  : 100  — max concurrent MongoDB operations
MaxIdleTime  : 30s  — drop unused connections after 30 seconds
ConnectTimeout: 10s — fail fast if MongoDB is unreachable on startup
```

---

## 13. Performance

### Presence Endpoints (In-Memory — Zero DB Cost)

| Endpoint | Operation | Typical Response Time |
|---|---|---|
| `POST /presence/heartbeat` | RAM write | 1–3 ms (network overhead) |
| `PATCH /presence` | RAM write | 1–3 ms |
| `GET /presence/:userId` | RAM read | 1–2 ms |

### User Endpoints (MongoDB)

| Endpoint | Operation | Typical Response Time |
|---|---|---|
| `POST /auth/anonymous` | DB read + optional write | 10–20 ms |
| `GET /users/me` | DB read | 5–10 ms |
| `PATCH /users/me` | DB write + read | 15–25 ms |
| `GET /users/:userId` | DB read | 5–10 ms |

### How to Benchmark

```bash
curl -o /dev/null -s \
  -w "Total: %{time_total}s | TTFB: %{time_starttransfer}s\n" \
  -X PATCH http://localhost:8080/api/v1/presence \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"isSearching": true}'
```

---

## 14. Roadmap

### Phase 2 — Matchmaking
- Background goroutine scans presence store for `isSearching = true` users
- Pairs two compatible users (by gender preference, interests)
- Sets `isMatched = true` and `currentMatchId` on both
- Notifies both clients via WebSocket

### Phase 3 — Chat
- `messages` collection: `{ chatId, senderId, text, sentAt, readAt }`
- WebSocket endpoint for real-time message delivery
- Read receipts and unread counts

### Phase 4 — Presence Persistence
- Periodic flush: RAM presence state → MongoDB every 60 seconds
- Preserves `lastSeen` across server restarts

### Phase 5 — Scale-Out
- Replace local RAM map with **Redis** for distributed presence
- Multiple backend instances can share one Redis store
- Redis Pub/Sub for cross-instance presence events

### Phase 6 — Premium Features
- `premiumTill` field is already in the User model
- Payment webhook sets `premiumTill = now + 30 days`
- Premium users get priority matching, advanced interest filters

---

## Quick Reference

```
# Health check (no auth needed)
GET  /health

# Auth (no auth needed)
POST /api/v1/auth/anonymous

# User profile (Bearer token required)
GET   /api/v1/users/me
PATCH /api/v1/users/me
GET   /api/v1/users/:userId

# Realtime presence (Bearer token required)
POST  /api/v1/presence/heartbeat   <- call every 20s while app is active
PATCH /api/v1/presence
GET   /api/v1/presence/:userId
```

---

*Built for Ranchat — anonymous connections, real conversations.*
