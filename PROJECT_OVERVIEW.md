# 🚀 RanChat - Comprehensive System Architecture & Detailed Technical Overview

**Project:** RanChat (Omegle-like Random Text/Video Chat Platform Backend Engine)  
**Tech Stack:** Go (Golang 1.26.1), Gin Framework v1.12, Gorilla WebSocket v1.5, Redis v9 (Presence Engine), MongoDB Driver v2 (Persistence Engine), JWT Authentication & Middleware, HTTP Long-Polling Matchmaking Engine, Domain Events (DDD), Exponential Backoff Retries  
**Last Updated:** September 29, 2026  
**Overall Architecture Rating:** **8.9 / 10** *(Clean Event-Driven Domain-Driven Monolith with Redis Presence, Long-Polling Matchmaking Engine, Active WebSocket Hub & Persistent Friendship System)*  
**Primary Objective:** High-throughput, sub-second latency user onboarding, Redis-backed status tracking, interest/gender-matched random pairing, real-time WebSocket notifications, persistent friendship connections, and scalable chat backend foundation.

---

## 📊 1. System Scorecard & Architectural Evaluation

| Domain Context | Rating | Status & Observations |
| :--- | :---: | :--- |
| **Architecture & DDD** | 🟢 **9.0 / 10** | Strict Bounded Context isolation between Auth, Presence, Matchmaking, Friendship, and WebSocket via [`events/dispatcher.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/events/dispatcher.go) & middleware. Zero circular imports. |
| **Eventual Consistency** | 🟢 **9.0 / 10** | Non-blocking signup HTTP handlers. MongoDB primary write succeeds first; Redis presence state converges asynchronously via event dispatching. |
| **Fault Tolerance & Retries** | 🟢 **8.5 / 10** | Production-standard retry system ([`utils/retries.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/utils/retries.go)) featuring exponential backoff & context cancellation safety. |
| **Authentication & Security** | 🟢 **8.5 / 10** | MongoDB v2 driver integration, JWT token generation & verification ([`internals/middleware/auth_middleware.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/middleware/auth_middleware.go)), monetization & premium status tracking. |
| **Presence System** | 🟢 **8.5 / 10** | High-performance Redis repository ([`redis_presence_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/presence/repository/redis_presence_repository.go)) active in `main.go`. Key schema `presence:<userID>` with 20s heartbeat renewal. |
| **Matchmaking Engine** | 🟢 **8.0 / 10** | HTTP Long-Polling engine ([`match_making_services.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/match_making/services/match_making_services.go)) with reciprocal criteria matching (Interest, Gender, GenderPref) and 30-second context timeout. Audit in [`matchmakingoverview.md`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/matchmakingoverview.md). |
| **Real-time & WebSockets** | 🟢 **8.5 / 10** | Fully operational WebSocket Hub ([`internals/websocket`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/websocket)) at `/v1/ws` enabling real-time connection tracking and direct JSON pushes (e.g. `friend_request_received`). |
| **Friendship System** | 🟡 **7.0 / 10** | MongoDB-backed Friendship module ([`internals/friendship`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship)) integrated with `WebSocketHub` for instant notifications. Audit in [`friendshipoverview.md`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/friendshipoverview.md). |

---

## 🏗️ 2. Core Architectural Design Patterns & Workflows

### Pattern A: Bounded Context Isolation (DDD)
The codebase is strictly organized into independent domain contexts:
1. **Authentication Context** ([`internals/authentication`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/authentication)): Manages user registration, login, profile updates, JWT issuance, and MongoDB persistence. Emits domain events (`UserSignedUpEvent`).
2. **Presence Context** ([`internals/presence`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/presence)): Manages online status, device IDs, heartbeats, and active chat states via Redis (`RedisPresenceRepository`). Listens to domain events without importing Auth controllers.
3. **Matchmaking Context** ([`internals/match_making`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/match_making)): Handles user pairing using in-memory request queues (`MatchmakingMemoryDB`), match tracking (`MatchMemoryDB`), and long-polling notification channels (`EnqueueChan`).
4. **WebSocket Context** ([`internals/websocket`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/websocket)): Manages active persistent WebSocket connections (`WebSocketHub`) at `/v1/ws` and enables real-time client pushes.
5. **Friendship Context** ([`internals/friendship`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship)): Manages asynchronous friend request workflows (send, accept, decline), active friend list management, and real-time WebSocket request popups (`WSHub.SendToUser`).
6. **Middleware Layer** ([`internals/middleware`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/middleware)): Centralized `AuthMiddleware` verifying Bearer JWT tokens and attaching `userId` to Gin context for protected HTTP and WebSocket routes.
7. **Event Bus** ([`events/dispatcher.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/events/dispatcher.go)): A lightweight shared pub/sub dispatcher bridging domain events asynchronously without package dependencies.

---

### Pattern B: Real-Time WebSocket Notification Flow

```
[ User A ]                                [ Server / WSHub ]                          [ User B ]
    │                                             │                                       │
    │ ─── 1. Connected to /v1/ws ───────────────► │ ◄─── 2. Connected to /v1/ws ───────── │
    │     Registered in `WebSocketHub`            │     Registered in `WebSocketHub`      │
    │                                             │                                       │
    │ ─── 3. POST /v1/friendship/requests ──────► │                                       │
    │     { "receiver_id": "UserB_ID" }           │ ──► Saved to MongoDB                  │
    │                                             │                                       │
    │                                             │ ─── 4. `WSHub.SendToUser(UserB_ID)` ─►│
    │ ◄── 5. HTTP 201 Created ─────────────────── │     `{"event":"friend_request"}`     │
```

---

## 📁 3. Detailed Package Breakdown & Code Anatomy

```text
ranchat/
├── cmd/
│   └── main.go                             # App bootstrapping, Redis/Mongo/WSHub setup, route registration
├── internals/
│   ├── websocket/
│   │   ├── hub.go                          # Thread-safe WebSocketHub mapping UserID -> *websocket.Conn
│   │   └── ws_controller.go                # HTTP upgrader handler for GET /v1/ws endpoint
│   ├── friendship/
│   │   ├── controllers/
│   │   │   └── friendship_controller.go   # Handlers using WSHub to push real-time friend requests
│   │   ├── dto/
│   │   ├── models/
│   │   ├── repository/
│   │   ├── routes/
│   │   └── services/
```

---

## 🌐 4. Complete API Endpoints Reference

### ⚡ WebSocket Context (`/v1/ws`)
| Method | Endpoint | Access | Description |
| :--- | :--- | :---: | :--- |
| `GET` | `/v1/ws` | Protected | Upgrades HTTP connection to WebSocket protocol, registers connection in `WebSocketHub`. |


---

### Pattern B: Asynchronous User Onboarding Flow
Rather than performing a blocking synchronous write to both MongoDB and Redis/Presence inside the HTTP signup handler:

```
[ POST /v1/auth/signup ]
         │
         ▼
 1. Insert User to Mongo DB (Primary Source of Truth)
         │
         ▼
 2. Generate JWT & Return HTTP 201 Created to Client (Sub-second response!)
         │
         ▼ (Asynchronous Goroutine)
 3. Publish `UserSignedUpEvent` to EventDispatcher
         │
         ▼
 4. Execute `utils.DoWithRetry` (3 Attempts: 100ms ➔ 200ms ➔ 400ms)
         │
         ▼
 5. `UserSignupHandler.Handle()` creates initial presence record in Redis (`presence:<userID>`)
```

---

### Pattern C: HTTP Long-Polling Matchmaking Architecture
The matchmaking subsystem uses reciprocal preference matching with an efficient long-polling mechanism:

```
[ POST /v1/matching/start ] (Protected by AuthMiddleware)
         │
         ▼
 1. Extract `userId` from Gin Context
         │
         ▼
 2. Call `MatchMakingService.MatchAndEnqueue()`
         │
  ┌──────┴────────────────────────────────────────┐
  ▼                                               ▼
[ Existing Candidate Found? ]           [ No Match Available ]
  │                                               │
  ├─► Create `Match` (ObjectID Hex)               ├─► Create `chan *models.Match` (buffer size 1)
  ├─► Delete candidate from pending queue         ├─► Save request in `MatchmakingMemoryDB`
  ├─► Push `Match` to candidate's channel         ├─► Register channel in `EnqueueChan` map
  └─► Return `Match` directly to current user     └─► Wait on `select` for up to 30 Seconds
                                                          │
                                         ┌────────────────┴────────────────┐
                                         ▼                                 ▼
                                 [ Match Received ]               [ 30s Timeout / Disconnect ]
                                         │                                 │
                                  Return HTTP 200                   Remove request & channel
                                  { status: "matched" }             Return HTTP 200
                                                                    { status: "retry" }
```

---

### Pattern D: Persistent Social Networking & Friendship Lifecycle

```
[ User A ]                                [ Server / MongoDB ]                       [ User B ]
    │                                              │                                     │
    │ ─── 1. POST /v1/friendship/requests ───────► │                                     │
    │     { "receiver_id": "UserB_ID" }            │ ──► Insert into `friendrequests`    │
    │                                              │                                     │
    │                                              │ ◄── 2. GET /v1/friendship/requests ─ │
    │                                              │     Return pending requests [...]   │
    │                                              │                                     │
    │                                              │ ◄── 3. POST /requests/:id/accept ─── │
    │                                              │     ├── Insert into `friends`       │
    │                                              │     └── Delete from `friendrequests`│
    │                                              │                                     │
    │ ◄── 4. GET /v1/friendship/friends ────────── │ ──► GET /v1/friendship/friends ───► │
    │     Return Active Friendships [...]          │     Return Active Friendships [...] │
```

---

### Pattern E: Resilient Retries & Concurrency Safety
- **Copy-on-Read Dispatching:** In [`events/dispatcher.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/events/dispatcher.go#L29-L33), the handler slice is snapshotted under `RUnlock()` before spawning goroutines, preventing lock contention during execution.
- **Context-Aware Exponential Backoff:** In [`utils/retries.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/utils/retries.go#L26-L58), `DoWithRetry` checks `select { case <-ctx.Done(): ... }` so application shutdowns cancel sleeping retries cleanly.
- **Thread-Safe Memory Structures:** All in-memory maps in Matchmaking (`Requests`, `Matches`, `Chans`) and Presence (`Presences`) are guarded by dedicated `sync.RWMutex` instances.

---

## 💾 3. Database Schemas & Persistence Model Specifications

### 1. MongoDB Database: `ranchat`

#### Collection A: `users`
Stores user profile information, interest tags, and premium monetization state.
```go
type User struct {
    UserId      bson.ObjectID `json:"user_id"     bson:"_id,omitempty"`
    Name        string        `json:"name"        bson:"name"`
    Age         int           `json:"age"         bson:"age"`
    Gender      string        `json:"gender"      bson:"gender"`      // "male", "female", "other"
    Bio         string        `json:"bio"         bson:"bio"`
    Interest    []string      `json:"interest"    bson:"interest"`   // e.g. ["coding", "gaming"]
    DeviceID    string        `json:"deviceId"    bson:"deviceId"`
    PremiumTill time.Time     `json:"premiumTill" bson:"premiumTill"` // nil / zero = free user
    CreatedAt   time.Time     `json:"createdAt"   bson:"createdAt"`
    UpdatedAt   time.Time     `json:"updatedAt"   bson:"updatedAt"`
}
```

#### Collection B: `friendrequests`
Stores pending, accepted, or declined friend requests between users.
```go
type FriendRequest struct {
    ID         bson.ObjectID `json:"id,omitempty"          bson:"_id,omitempty"`
    SenderID   bson.ObjectID `json:"sender_id"             bson:"sender_id"`
    ReceiverID bson.ObjectID `json:"receiver_id"           bson:"receiver_id"`
    Status     string        `json:"status"                bson:"status"` // "pending", "accepted", "declined"
    CreatedAt  time.Time     `json:"created_at"            bson:"created_at"`
    UpdatedAt  time.Time     `json:"updated_at"            bson:"updated_at"`
}
```

#### Collection C: `friends`
Stores active 2-way friendship relationships.
```go
type Friend struct {
    ID        bson.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
    UserIDs   []bson.ObjectID `json:"user_ids"     bson:"user_ids"` // Contains [UserA_ID, UserB_ID]
    CreatedAt time.Time       `json:"created_at"   bson:"created_at"`
}
```

---

### 2. Redis Key Storage Schema

#### Key Pattern: `presence:<userID>`
Stores active presence state, device ID, heartbeat timestamps, typing indicator, and active chat session.
```json
{
  "userId": "66fa54122b11d8c11e74a812",
  "deviceId": "dev-device-xyz-123",
  "lastActiveAt": "2026-09-29T10:30:00Z",
  "activeChatId": null,
  "typingTo": null,
  "updatedAt": "2026-09-29T10:30:00Z"
}
```

---

### 3. In-Memory Memory Databases (Matchmaking Context)

```go
type MatchmakingMemoryDB struct {
    Mu       sync.RWMutex
    Requests map[string]*dto.MatchmakingRequest // Key: UserID
}

type MatchMemoryDB struct {
    Mu      sync.RWMutex
    Matches map[string]*models.Match // Key: MatchID (Hex)
}

type EnqueueChan struct {
    Mu    sync.RWMutex
    Chans map[string]chan *models.Match // Key: UserID
}
```

---

## 📁 4. Detailed Package Breakdown & Code Anatomy

```text
ranchat/
├── .env                                    # Environment configurations (PORT, MONGO_URI, JWT_SECRET)
├── errors.go                               # Centralized sentinel errors (ErrUserNotFound, ErrPresenceExists, etc.)
├── go.mod                                  # Go dependencies (Gin v1.12, Mongo v2.8, Redis v9.22, JWT v5.3)
├── go.sum                                  # Module checksums
├── package.json                            # Helper meta configuration
├── PROJECT_OVERVIEW.md                     # Complete System Architecture & Documentation (THIS FILE)
├── matchmakingoverview.md                  # Comprehensive technical audit of matchmaking engine
├── friendshipoverview.md                   # Comprehensive technical overview & audit of friendship subsystem
├── cmd/
│   └── main.go                             # App bootstrapping, Redis/Mongo setup, route registration, Gzip middleware
├── config/
│   └── config.go                           # Env variable loader & Mongo connection pool setup (Max: 300, Min: 50)
├── events/
│   ├── events.go                           # Domain event structs (UserSignedUpEvent) & topic constants
│   └── dispatcher.go                       # Thread-safe pub/sub event dispatcher with retry integration
├── utils/
│   └── retries.go                          # Resilient exponential backoff engine (DoWithRetry)
└── internals/
    ├── authentication/
    │   ├── controller/
    │   │   └── user_controller.go          # Auth HTTP Handlers (SignUpUser, GetUser)
    │   ├── dto/
    │   │   └── errors.go                  # Auth ErrorResponse DTO
    │   ├── models/
    │   │   └── user_model.go             # User domain model (BSON & JSON tags, Gender enum)
    │   ├── repository/
    │   │   └── user_repository.go          # UserRepository interface & MongoDB implementation
    │   ├── routes/
    │   │   └── auth_routes.go              # Public & protected auth route definitions
    │   ├── service/
    │   │   └── user_service.go            # UserService business logic & event publishing
    │   └── utils/
    │       └── jwt.go                     # JWT token generation & parsing utilities
    ├── friendship/
    │   ├── controllers/
    │   │   └── friendship_controller.go   # HTTP handlers for friend requests and friend list CRUD
    │   ├── dto/
    │   │   └── friend_request             # Placeholder DTO file (0 bytes)
    │   ├── models/
    │   │   ├── friend.go                  # Friend domain model (UserIDs array, CreatedAt)
    │   │   └── friend_request.go          # FriendRequest domain model (SenderID, ReceiverID, Status)
    │   ├── repository/
    │   │   └── friendship_repository.go   # MongoDB storage layer for `friends` and `friendrequests`
    │   ├── routes/
    │   │   └── friendship_routes.go       # Route definitions for `/v1/friendship/*`
    │   └── services/
    │       └── friendship_services.go     # Business service layer for friendship workflows
    ├── match_making/
    │   ├── controllers/
    │   │   └── match_making_controller.go # StartMatching HTTP endpoint handler (30s long-polling)
    │   ├── database/
    │   │   ├── matchmaking_requests_db.go # Thread-safe MatchmakingMemoryDB, MatchMemoryDB & EnqueueChan
    │   │   └── match_memory.go            # Legacy database reference file
    │   ├── dto/
    │   │   └── match_making_request.go    # MatchmakingRequest payload (Interest, Gender, GenderPref)
    │   ├── models/
    │   │   └── match.go                   # Match model (MatchID, UserIDs [2]string, CreatedAt)
    │   ├── repository/
    │   │   └── match_making_repository.go # Reciprocal matching queries & atomic map operations
    │   ├── routes/
    │   │   └── match_making_routes.go     # /v1/matching route registration
    │   └── services/
    │       └── match_making_services.go   # MatchAndEnqueue algorithm & channel notification logic
    ├── middleware/
    │   └── auth_middleware.go             # JWT Bearer token validation middleware
    └── presence/
        ├── controller/
        │   └── presence_controller.go     # Presence HTTP Handlers (UpsertPresence, HeartBeat, UpdatePresence)
        ├── database/
        │   └── presence_database.go       # In-memory presence storage map
        ├── dto/
        │   └── update_presence.go         # UpdatePresenceFields DTO (TypingTo, ActiveChatID)
        ├── handler/
        │   └── user_signup_handler.go     # Async event handler reacting to UserSignedUp
        ├── models/
        │   └── presence_model.go          # Presence state domain model
        ├── repository/
        │   ├── presence_repository.go     # PresenceRepository interface & InMemory implementation
        │   └── redis_presence_repository.go# Production Redis implementation (`presence:<userID>`)
        ├── routes/
        │   └── presence_routes.go         # /v1/presence route registration
        └── services/
            └── presence_service.go        # Presence business logic (Upsert, Heartbeat, Update)
```

---

## 🔍 5. Key Component Deep-Dives

### 1. [`internals/presence/repository/redis_presence_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/presence/repository/redis_presence_repository.go)
- **Redis Key Scheme:** `presence:<userID>`
- **Operations:**
  - `SavePresence`: Serializes `models.Presence` to JSON and executes `SetNX` to guarantee creation uniqueness.
  - `Heartbeat`: Retrieves presence, updates `LastActiveAt` and `UpdatedAt` timestamps, and persists back to Redis.
  - `UpdatePresence`: Partial update for active chat context (`ActiveChatID`) and typing indicators (`TypingTo`).

### 2. [`internals/match_making/services/match_making_services.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/match_making/services/match_making_services.go)
- **Matching Criteria:** Evaluates reciprocal compatibility:
  - `candidate.Interest == req.Interest`
  - `candidate.Gender == req.GenderPref`
  - `candidate.GenderPref == req.Gender`
- **Channel Dispatch:** If a candidate is waiting, `MatchAndEnqueue` builds a `Match`, notifies candidate via `EnqueueChan.Chans[candidate.UserID] <- match`, and returns a single-item buffered channel to the caller.

### 3. [`internals/friendship/repository/friendship_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go)
- **MongoDB Collections:** `friends` and `friendrequests`
- **Operations:**
  - `SendFriendRequest`: Verifies no existing request exists between `sender_id` and `receiver_id`, then inserts a `models.FriendRequest` object.
  - `AcceptFriendRequest`: Fetches request by `_id`, creates a `models.Friend` document with both user ObjectIDs in `user_ids`, inserts it into `friends`, and deletes the request from `friendrequests`.
  - `DeclineFriendRequest`: Deletes request by `_id` from `friendrequests`.
  - `RemoveFriend`: Deletes friendship document by `_id` from `friends`.
  - `GetAllFriendRequest`: Fetches pending requests where `receiver_id` matches the user's ObjectID.
  - `GetAllFriends`: Fetches friendships where `user_ids` contains the user's ObjectID.

### 4. [`internals/middleware/auth_middleware.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/middleware/auth_middleware.go)
- **Token Extraction:** Parses `Authorization: Bearer <token>` header.
- **Verification:** Calls `utils.VerifyJWTToken(tokenStr)`. On success, sets `userId` into Gin context (`ctx.Set("userId", claims.ID)`). Aborts request chain with HTTP 401 on missing or invalid tokens.

### 5. [`events/dispatcher.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/events/dispatcher.go)
- **`EventHandler` Signature:** `func(payload any) error`
- **Asynchronous Execution:** Spawns background goroutine per subscriber executing `utils.DoWithRetry`. Returns error if execution fails after max attempts.

---

## 🌐 6. Complete API Endpoints Reference

### 🔐 Authentication Context (`/v1/auth`)
| Method | Endpoint | Access | Request Body | Success Response | Description |
| :--- | :--- | :---: | :--- | :--- | :--- |
| `POST` | `/v1/auth/signup` | Public | `User` JSON fields | `201 Created`<br>`{"token": "JWT..."}` | Registers user in MongoDB, returns JWT token, fires `UserSignedUpEvent`. |
| `GET` | `/v1/auth/user` | Protected | None | `200 OK`<br>`{"user": {...}}` | Retrieves current user profile from MongoDB using JWT `userId`. |

### 🟢 Presence Context (`/v1/presence`)
| Method | Endpoint | Access | Request Body | Success Response | Description |
| :--- | :--- | :---: | :--- | :--- | :--- |
| `POST` | `/v1/presence` | Protected | `{"deviceId": "..."}` | `201 Created` | Initializes presence record in Redis (`presence:<userID>`). |
| `POST` | `/v1/presence/heartbeat` | Protected | None | `200 OK` | Updates `LastActiveAt` timestamp in Redis. |
| `PATCH` | `/v1/presence/update` | Protected | `UpdatePresenceFields` | `200 OK` | Updates typing state (`typingTo`) or active chat session (`activeChatId`). |

### ⚡ Matchmaking Context (`/v1/matching`)
| Method | Endpoint | Access | Request Body | Success Response | Description |
| :--- | :--- | :---: | :--- | :--- | :--- |
| `POST` | `/v1/matching/start` | Protected | `MatchmakingRequest` | `200 OK`<br>`{"status": "matched"}` | Enters matching pool with criteria (Interest, Gender, GenderPref). Long-polls up to 30s. |

### 🤝 Friendship Context (`/v1/friendship`)
| Method | Endpoint | Access | Request Body | Success Response | Description |
| :--- | :--- | :---: | :--- | :--- | :--- |
| `POST` | `/v1/friendship/requests` | Protected | `{"receiver_id": "hex"}` | `201 Created` | Sends a friend request. |
| `POST` | `/v1/friendship/requests/:requestId/accept` | Protected | None | `200 OK` | Accepts an incoming friend request by request ID. |
| `POST` | `/v1/friendship/requests/:requestId/decline` | Protected | None | `200 OK` | Declines an incoming friend request by request ID. |
| `GET` | `/v1/friendship/requests` | Protected | None | `200 OK`<br>`{"requests": [...]}` | Retrieves all incoming pending friend requests for authenticated user. |
| `GET` | `/v1/friendship/friends` | Protected | None | `200 OK`<br>`{"friends": [...]}` | Retrieves all active friendships for authenticated user. |
| `DELETE` | `/v1/friendship/friends/:friendshipID` | Protected | None | `200 OK` | Removes an existing friend by friendship ID. |

---

## 🔬 7. Technical Audits & Identified Module Vulnerabilities

### 🎯 Matchmaking Engine Audit ([`matchmakingoverview.md`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/matchmakingoverview.md))
- **Score:** **6.5 / 10**
- **Critical Vulnerabilities:**
  1. **TOCTOU Race Condition:** Non-atomic check-then-act in `FindMatch` leading to potential double-matching under high concurrency.
  2. **Ghost Match Bug:** Context timeout race condition where a matched user receives a match notification after their long-poll timed out.
  3. **$O(N)$ Map Iteration:** Unindexed map iteration over pending requests causing high CPU utilization under load.

### 🤝 Friendship System Audit ([`friendshipoverview.md`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/friendshipoverview.md))
- **Score:** **6.6 / 10**
- **Critical Vulnerabilities:**
  1. **IDOR / Missing Authorization:** `AcceptFriendRequest`, `DeclineFriendRequest`, and `RemoveFriend` do not verify if `authenticatedUserID` owns or is part of the request/friendship.
  2. **Reciprocal Validation Deficiencies:** Missing checks for self-friend requests, reverse pending requests, or existing active friendships.
  3. **Un-indexed MongoDB Scans:** Missing compound indexes on `friendrequests` (`receiver_id`, `sender_id`) and `friends` (`user_ids`).

---

## 🛣️ 8. Comprehensive Engineering Roadmap & Milestones

1. **Phase 1: Security & IDOR Authorization Fixes**
   - Add ownership validation checks in `AcceptFriendRequest`, `DeclineFriendRequest`, and `RemoveFriend` so users can only accept/decline requests sent to them and remove friendships they belong to.
   - Add MongoDB compound indexes on `friendrequests` (`receiver_id`, `sender_id`) and `friends` (`user_ids`).

2. **Phase 2: Atomic In-Memory Matchmaking Engine**
   - Combine candidate lookup and queue removal into a single atomic function under a full `Write Lock` (`Mu.Lock()`).
   - Implement category bucketing (`MatchmakingBuckets`) for $O(1)$ interest queue lookups.

3. **Phase 3: Real-Time WebSocket Hub (`/v1/ws`)**
   - Upgrade HTTP connection to WebSocket protocol using `github.com/gorilla/websocket`.
   - Implement `WebSocketHub` to maintain connection registry for real-time friend request popups, typing indicators, and instant text messaging.

4. **Phase 4: Distributed Matchmaking Engine (Redis Pub/Sub & Streams)**
   - Transition `MatchmakingMemoryDB` and `EnqueueChan` to Redis Pub/Sub / Redis Streams to enable multi-instance horizontal backend scaling.

5. **Phase 5: WebRTC Audio/Video SDP Signaling**
   - Exchange WebRTC offers, answers, and ICE candidates via WebSockets for peer-to-peer audio and video streaming.
