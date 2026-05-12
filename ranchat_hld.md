# Ranchat Backend — High-Level Design (HLD)

> **Stack:** Go · Gin · MongoDB · gorilla/websocket  
> **Server:** Single-process, single-instance (horizontal scaling via Redis pub/sub is planned)  
> **Port:** `8080` (configured via `PORT` env var)

---

## 1. System Overview

Ranchat is an **anonymous random-chat platform**. Two strangers are paired in real time, can chat, and optionally exchange a friend request. The backend is built for **low latency, high concurrency, and zero-DB writes on the hot path**.

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Mobile Client (Flutter)                      │
│                                                                     │
│  REST (JSON/HTTP)          Long-Poll (HTTP)       WebSocket         │
│  Auth, Profile, Friends    Matchmaking            Match Session      │
└────────┬──────────────────────────┬──────────────────┬─────────────┘
         │                          │                  │
         ▼                          ▼                  ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    Gin HTTP Server (port 8080)                       │
│                                                                      │
│  ┌─────────────┐  ┌──────────────────────┐  ┌───────────────────┐  │
│  │ JWT Auth    │  │  Long-Poll Handler   │  │  WS Upgrade       │  │
│  │ Middleware  │  │  (goroutine blocks   │  │  Handler          │  │
│  │             │  │   on waitCh ≤ 30s)   │  │  /ws/match/:id   │  │
│  └─────────────┘  └──────────────────────┘  └───────────────────┘  │
│         │                    │                        │              │
│         └────────────────────┴────────────────────────┘              │
│                              │                                       │
│                    ┌─────────▼──────────┐                           │
│                    │   Service Layer     │                           │
│                    │  (business logic)   │                           │
│                    └─────────┬──────────┘                           │
│                              │                                       │
│          ┌───────────────────┼────────────────────┐                 │
│          ▼                   ▼                    ▼                  │
│  ┌──────────────┐  ┌─────────────────┐  ┌──────────────────┐       │
│  │   MongoDB    │  │   In-Memory     │  │   In-Memory      │       │
│  │  (disk)      │  │  Match Queue +  │  │  WS Room Store   │       │
│  │  users       │  │  Active Matches │  │  (matchID →      │       │
│  │  friend_reqs │  │  Presence Store │  │   WSClient pair) │       │
│  │  friendships │  │  Waiter Channels│  │                  │       │
│  └──────────────┘  └─────────────────┘  └──────────────────┘       │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 2. Layered Architecture

The codebase follows a strict **4-layer architecture** with no dependency inversion (flat DI-free style):

```
┌─────────────────────────────────────────────────┐
│              Layer 1: Routes                    │
│   routes/routes.go — URL → Controller mapping  │
└───────────────────────┬─────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────┐
│              Layer 2: Controller                │
│   Parse HTTP request → call Service → respond  │
│   controller/user_controller.go                │
│   controller/match_controller.go               │
│   controller/presence_controller.go            │
│   controller/friend_controller.go              │
│   controller/ws_controller.go                  │
└───────────────────────┬─────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────┐
│              Layer 3: Service                   │
│   Business logic, orchestration, validation    │
│   service/user_service.go                      │
│   service/match_service.go                     │
│   service/presence_service.go                  │
│   service/friend_service.go                    │
└───────────────────────┬─────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────┐
│              Layer 4: Repository                │
│   Data access — RAM or MongoDB                 │
│   repository/user_repository.go    (MongoDB)   │
│   repository/presence_repository.go (RAM)      │
│   repository/match_repository.go   (RAM)       │
│   repository/friendship_repository.go (MongoDB)│
│   repository/ws_room_repository.go (RAM)       │
└─────────────────────────────────────────────────┘
```

**Rule:** Each layer only calls the layer directly below it. Controllers never touch MongoDB; Services never touch `gin.Context`.

---

## 3. Data Storage — What Lives Where

| Data | Storage | Why |
|---|---|---|
| User profiles | **MongoDB** `users` | Stable, infrequently updated, survives restarts |
| Friend requests | **MongoDB** `friend_requests` | Persistent social graph data |
| Friendships | **MongoDB** `friendships` | Permanent accepted connections |
| Presence state | **RAM** (`presenceStore`) | Changes 1000s× per second; irrelevant after restart |
| Match queue | **RAM** (`genderPartitions`) | Hot path — needs nanosecond access |
| Active matches | **RAM** (`activeMatches`) | Ephemeral session state |
| Long-poll channels | **RAM** (`waiterRegistry`) | Goroutine synchronization primitives |
| WebSocket rooms | **RAM** (`wsRooms`) | Active session routing only |

---

## 4. Core In-Memory Data Structures

### 4.1 Partitioned Match Queue

```
genderPartitions[3]
│
├── [0] male shard   → [ WaitingUser{A}, WaitingUser{C}, ... ]
│        mutex ──────────── protects this slice only
│
├── [1] female shard → [ WaitingUser{B}, ... ]
│        mutex ──────────── protects this slice only
│
└── [2] other shard  → [ WaitingUser{D}, ... ]
         mutex ──────────── protects this slice only

queuedSet (sync.Map)  →  { userID: {} }   ← O(1) duplicate detection
```

**Lock order rule** (deadlock prevention): shards are always locked in ascending index order `0 → 1 → 2`. `activeMatches.mu` is never held while any shard lock is held.

### 4.2 Waiter Registry (Long-Poll Channels)

```
waiterRegistry.m
│
├── "userA_id" → chan MatchResult (buffered, size 1)
├── "userB_id" → chan MatchResult
└── ...

HTTP handler goroutine blocks on this channel (select { case result := <-waitCh })
storeMatch() calls notifyWaiter() → sends MatchResult → handler unblocks → 200 OK
```

### 4.3 Active Match Store

```
activeMatches
│
├── matches: { matchID → *Match{User1, User2, StartedAt} }
└── byUser:  { userID → matchID }   ← O(1) lookup by user
```

### 4.4 WebSocket Room Store

```
wsRooms
│
└── rooms: {
      matchID → {
        user1ID → *WSClient{ conn, send chan []byte }
        user2ID → *WSClient{ conn, send chan []byte }
      }
    }
```

Each `WSClient` has:
- A **ReadPump goroutine** (blocking `conn.ReadMessage()`)
- A **WritePump goroutine** (draining the `send` channel)

---

## 5. Authentication Flow

```
Client                           Server
  │                                │
  │── POST /auth/anonymous ────────▶│
  │   { deviceId, fullName, gender,│
  │     age, about, interests }    │
  │                                │── FindUserByDeviceID (MongoDB)
  │                                │   ├── Not found → Create user (201)
  │                                │   └── Found     → Login    (200)
  │                                │── Sign JWT (HS256, 720h TTL)
  │                                │── UpsertPresence in RAM
  │◀── { token, expiresAt, ────────│
  │      user, presence }          │
  │                                │
  │   (saves token for all future requests)
```

**JWT payload:** `{ userID, deviceID, exp }`  
**Auth middleware** reads `Authorization: Bearer <token>`, validates, injects `userID` into `gin.Context`.  
**WebSocket** uses `?token=<jwt>` query param (browsers can't set headers in WS handshake).

---

## 6. Matchmaking — Long-Polling Design

This is the most complex part of the system.

```
User A                          Server                         User B
  │                               │                               │
  │── POST /matches/search ───────▶│                               │
  │   { wantGender: "female" }    │                               │
  │                               │── SetPresence isSearching=true (RAM)
  │                               │── Fetch User A gender from MongoDB
  │                               │── RegisterWaiter(A) → make(chan, 1)
  │                               │── Enqueue(A) → scan female shard
  │                               │   └── Queue empty → push A into male shard
  │                               │── controller blocks on: select { case <-waitCh }
  │   (request hangs ≤ 30s...)    │
  │                               │                               │
  │                               │◀── POST /matches/search ──────│
  │                               │    { wantGender: "anyone" }   │
  │                               │                               │
  │                               │── Enqueue(B) → scan all shards
  │                               │   └── Finds User A (compatible!)
  │                               │── Remove A from queue
  │                               │── storeMatch(B, A):
  │                               │    ├── Create Match{matchID, A, B}
  │                               │    ├── activeMatches[matchID] = Match
  │                               │    ├── byUser[A] = matchID
  │                               │    └── byUser[B] = matchID
  │                               │── notifyWaiter(A) → sends to A's channel
  │                               │── SetMatched(A, B) in Presence RAM
  │                               │── SetMatched(B, A) in Presence RAM
  │                               │
  │◀── 200 { matchId, partnerId}──│── 200 { matchId, partnerId} ──▶│
  │   (A's blocked request        │   (B's request returns         │
  │    resolves via the channel)   │    immediately)                │
```

**Timeout path:** If no partner arrives within 30s, the controller's `time.After(30s)` fires → `202 "still searching..."` → client re-calls the endpoint.

**Compatibility check (mutual):**
- Does **incoming** user accept **candidate**'s gender? AND
- Does **candidate** accept **incoming**'s gender?
Both must be true to prevent one-sided mismatches.

---

## 7. WebSocket — Active Match Session

```
Matched User A                 Server                Matched User B
     │                           │                           │
     │── GET /ws/match/:id ──────▶│                           │
     │   ?token=<jwt>             │── Validate JWT            │
     │                           │── Confirm user in match   │
     │                           │── Upgrade HTTP→WS         │
     │                           │── JoinRoom(matchID, A, conn)
     │                           │── go client.WritePump()    │
     │                           │── client.ReadPump() blocks │
     │                           │                           │
     │── { type: "SEND_FRIEND_REQUEST" } ──────────────────▶ │ (same room)
     │                           │── service.SendFriendRequest()
     │                           │   ├── Persist to MongoDB friend_requests
     │                           │   └── GetPartner(matchID, A) → B's WSClient
     │                           │── SendToClient(B, FRIEND_REQUEST_RECEIVED)
     │                           │──────────────────────────▶│
     │                           │                    receives event
     │                           │
     │◀── FRIEND_REQUEST_SENT ───│  (echo back to A as confirmation)
     │                           │
     │                           │◀── { type: "ACCEPT_FRIEND_REQUEST",
     │                           │      data: { requestId: "..." } } ──────│
     │                           │── service.ResolveFriendRequest()
     │                           │   ├── Update friend_request.status = accepted
     │                           │   └── Create Friendship in MongoDB
     │                           │── SendToClient(A, FRIEND_REQUEST_ACCEPTED)
     │◀── FRIEND_REQUEST_ACCEPTED│
     │                           │
     │── (disconnect)            │── LeaveRoom(matchID, A)
     │                           │── GetPartner() → B still connected
     │                           │── SendToClient(B, PARTNER_DISCONNECTED)
     │                           │──────────────────────────▶│
```

**Message envelope (both directions):**
```json
{ "type": "EVENT_TYPE_STRING", "data": { ... } }
```

---

## 8. Presence System

Presence state is **entirely in RAM**. It tracks fast-changing realtime state separately from stable profile data in MongoDB.

```
Presence {
  userID         ObjectID
  deviceID       string
  lastActiveAt   time.Time   ← updated every heartbeat
  isSearching    bool
  isMatched      bool
  currentMatchID *ObjectID
  activeChatId   *string
  typingTo       *string
  updatedAt      time.Time
}

isOnline() → time.Since(lastActiveAt) < 30s
```

**Heartbeat mechanism:**
- Client sends `POST /presence/heartbeat` every **~20 seconds**
- Server updates `lastActiveAt` in RAM (nanosecond operation)
- `isOnline()` derives online status from this timestamp — no disconnect events needed

---

## 9. Friend Request Lifecycle

```
State Machine:

    [pending] ──accept──▶ [accepted] ──creates──▶ Friendship (MongoDB)
        │
       reject
        │
        ▼
    [rejected]

MongoDB Collections:
  friend_requests { _id, matchId, fromId, toId, status, createdAt, updatedAt }
  friendships     { _id, user1Id, user2Id, matchId, createdAt }
```

**Delivery paths (dual):**
1. **Primary:** WebSocket push to connected partner (`FRIEND_REQUEST_RECEIVED`)
2. **Fallback:** `GET /friends/requests` polling for offline/reconnecting users

---

## 10. API Surface Summary

| Category | Method | Path | Auth | Transport |
|---|---|---|---|---|
| **Auth** | POST | `/api/v1/auth/anonymous` | ❌ Public | REST |
| **Health** | GET | `/health` | ❌ Public | REST |
| **Users** | GET | `/api/v1/users/me` | ✅ JWT | REST |
| | PATCH | `/api/v1/users/me` | ✅ JWT | REST |
| | GET | `/api/v1/users/:userId` | ✅ JWT | REST |
| **Presence** | POST | `/api/v1/presence/heartbeat` | ✅ JWT | REST |
| | PATCH | `/api/v1/presence` | ✅ JWT | REST |
| | GET | `/api/v1/presence/:userId` | ✅ JWT | REST |
| **Matchmaking** | POST | `/api/v1/matches/search` | ✅ JWT | **Long-Poll** |
| | DELETE | `/api/v1/matches/search` | ✅ JWT | REST |
| | GET | `/api/v1/matches/me` | ✅ JWT | REST |
| | DELETE | `/api/v1/matches/me` | ✅ JWT | REST |
| **Friends** | POST | `/api/v1/friends/requests` | ✅ JWT | REST |
| | GET | `/api/v1/friends/requests` | ✅ JWT | REST |
| | POST | `/api/v1/friends/requests/:id/accept` | ✅ JWT | REST |
| | POST | `/api/v1/friends/requests/:id/reject` | ✅ JWT | REST |
| | GET | `/api/v1/friends` | ✅ JWT | REST |
| **WebSocket** | GET | `/ws/match/:matchId?token=<jwt>` | ✅ JWT (query) | **WebSocket** |

---

## 11. Complete Client Lifecycle

```
App Launch
    │
    ▼
POST /auth/anonymous  →  receive JWT + userID
    │
    ▼
POST /presence/heartbeat  (repeat every 20s in background)
    │
    ▼
POST /matches/search  →  hangs ≤ 30s
    │         ├── 200 { matchId, partnerId }  →  match found!
    │         └── 202 "still searching"       →  retry loop
    │
    ▼ (match found)
GET /ws/match/:matchId?token=<jwt>  →  WebSocket connected
    │
    ├── Send:    { type: "SEND_FRIEND_REQUEST" }
    ├── Receive: { type: "FRIEND_REQUEST_RECEIVED", data: { requestId, fromId } }
    ├── Send:    { type: "ACCEPT_FRIEND_REQUEST", data: { requestId } }
    ├── Receive: { type: "FRIEND_REQUEST_ACCEPTED", data: { friendshipId } }
    └── Receive: { type: "PARTNER_DISCONNECTED" }
    │
    ▼ (match ends)
DELETE /matches/me  →  cleanup
    │
    ▼
GET /friends  →  view all friendships
```

---

## 12. Concurrency & Thread-Safety Model

| Component | Mechanism | Guarantee |
|---|---|---|
| Presence store | `sync.RWMutex` | Multiple readers OR one writer |
| Match queue shards | Per-shard `sync.Mutex` | Independent locks → no global bottleneck |
| Active match store | `sync.RWMutex` | Fast read-path for reconnection |
| Duplicate enqueue guard | `sync.Map` (atomic) | O(1) CAS — lock-free |
| Waiter registry | `sync.Mutex` + buffered channels (size 1) | Non-blocking send, never deadlocks |
| WS room store | `sync.RWMutex` | Safe concurrent joins/leaves |
| WS send queue | Buffered `chan []byte` (size 64) | Decouples ReadPump from WritePump |

**Goroutine model per matched user:**
- 1 **ReadPump** goroutine — blocks on `conn.ReadMessage()`
- 1 **WritePump** goroutine — drains `send` channel

---

## 13. MongoDB Schema & Indexes

### Collection: `users`
```
{
  _id:         ObjectId,
  deviceId:    string,      ← UNIQUE index
  fullName:    string,
  gender:      string,      ← index (for future queries)
  age:         int,
  about:       string,
  interests:   [string],
  premiumTill: Date | null,
  lastSeen:    Date,
  createdAt:   Date,
  updatedAt:   Date
}
Indexes: { deviceId: 1 } UNIQUE
```

### Collection: `friend_requests`
```
{
  _id:       ObjectId,
  matchId:   string,
  fromId:    ObjectId,
  toId:      ObjectId,      ← index (for GET /friends/requests)
  status:    string,        ← "pending" | "accepted" | "rejected"
  createdAt: Date,
  updatedAt: Date
}
Indexes: { toId: 1, status: 1 }
```

### Collection: `friendships`
```
{
  _id:       ObjectId,
  user1Id:   ObjectId,
  user2Id:   ObjectId,
  matchId:   string,
  createdAt: Date
}
Indexes: { user1Id: 1 }, { user2Id: 1 }
```

---

## 14. Current Limitations & Scale Boundaries

| Limitation | Impact | Future Fix |
|---|---|---|
| All state is in-process RAM | Single-server only — horizontal scaling breaks matchmaking | Redis pub/sub for queue + match state |
| No message history | Chat messages are not persisted | Add `messages` collection or Redis stream |
| No WebRTC signaling | No voice/video capability | Add signaling WS events |
| Presence resets on restart | All users appear offline after deploy | Redis for presence TTL keys |
| 30s long-poll timeout | Client must retry loop manually | Could use SSE for persistent stream |
| JWT is 720h (30 days) | No token revocation | Add refresh token + blocklist |
| No rate limiting | Susceptible to queue spam | Add middleware (e.g. `golang.org/x/time/rate`) |
