# Ranchat Backend — Complete System Design Document
> Last updated: May 2026 | Stack: Go + Gin + MongoDB + Gorilla WebSocket

---

## PART 1 — HIGH-LEVEL DESIGN (HLD)

### 1.1 System Purpose

Ranchat is a real-time anonymous matchmaking backend. Two strangers are paired together, enter a shared chat session, and optionally send each other friend requests before parting ways. The primary design goals are:

- **High concurrency** — support thousands of simultaneous users on a single process
- **Low latency** — match two users in milliseconds, not seconds
- **Minimal database pressure** — fast-changing state lives in RAM, not MongoDB

---

### 1.2 Architecture Pattern

The system uses a **Layered Monolith** architecture with a dual-storage model.

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENT (Mobile App)                     │
└────────────┬────────────────────────────────┬───────────────────┘
             │  HTTP / Long-Poll               │  WebSocket
             ▼                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GIN HTTP SERVER (:8080)                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐ │
│  │   Middleware  │  │   Routes     │  │  WS Upgrade Handler   │ │
│  │  JWT Auth     │  │  /api/v1/*   │  │  /ws/match/:matchId   │ │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬────────────┘ │
└─────────┼─────────────────┼─────────────────────┼──────────────┘
          ▼                 ▼                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                     CONTROLLER LAYER                            │
│  user_controller  match_controller  presence_controller         │
│  friend_controller                  ws_controller               │
└─────────────────────────────┬───────────────────────────────────┘
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      SERVICE LAYER                              │
│  user_service   match_service   presence_service  friend_service│
└──────────────────┬──────────────────────────────────────────────┘
                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                    REPOSITORY LAYER                             │
│  user_repository    friendship_repository   (→ MongoDB)         │
│  presence_repository  match_repository                          │
│  ws_room_repository                         (→ RAM only)        │
└──────────┬───────────────────────────────────────┬─────────────┘
           ▼                                       ▼
┌──────────────────────┐               ┌──────────────────────────┐
│      MONGODB         │               │      PROCESS RAM         │
│  - users             │               │  - presenceStore         │
│  - friend_requests   │               │  - matchQueue            │
│  - friendships       │               │  - activeMatches         │
└──────────────────────┘               │  - waiterRegistry        │
                                       │  - wsRooms               │
                                       └──────────────────────────┘
```

---

### 1.3 Dual Storage Strategy (The Core Design Decision)

| Data Type | Where Stored | Why |
|---|---|---|
| User profiles | MongoDB | Permanent, rarely changes |
| Friend requests | MongoDB | Must survive restarts, legal record |
| Friendships | MongoDB | Permanent social graph |
| Presence (online, searching, typing) | RAM | Changes 1000s of times/min; DB writes wasteful |
| Active matches | RAM | Lifecycle tied to current server session |
| WS match rooms | RAM | Alive only while both users are connected |
| Matchmaking queue | RAM | Volatile; destroyed on restart anyway |

---

### 1.4 Two Realtime Protocols — Strict Separation

```
LONG-POLLING ──────── ONLY for matchmaking
                      POST /matches/search blocks until a partner arrives

WEBSOCKET ─────────── ONLY for active matched sessions
                      ws://host/ws/match/:matchId
                      Handles: friend requests, typing, disconnect events
```

This separation ensures WebSocket resources are **only consumed by users currently in a match**, not by every searching user.

---

### 1.5 Request Flow Summary

```
1. Anonymous Login
   POST /auth/anonymous → MongoDB upsert → JWT returned → Presence seeded in RAM

2. Matchmaking (Long-Poll)
   POST /matches/search → Mark isSearching=true in RAM → Enter queue
   [Blocks up to 30s waiting for partner]
   When partner arrives → match created in RAM → 200 OK to both

3. Active Match (WebSocket)
   GET /ws/match/:matchId → Validate JWT + match ownership → Join room in RAM
   [Bidirectional events over WebSocket]

4. Friend Request
   WS: SEND_FRIEND_REQUEST → Validate match → Write to MongoDB → Push WS event to partner

5. Match Ends
   DELETE /matches/me → Clear RAM state → Delete WS room → Partner disconnects
```

---

## PART 2 — LOW-LEVEL DESIGN (LLD)

### 2.1 Authentication System

**File:** `service/user_service.go`, `middleware/auth.go`, `utils/jwt.go`

```
AnonymousLogin flow:
  deviceId ──→ FindUserByDeviceID (MongoDB)
                    │
         ┌──────────┴──────────┐
      exists?               not found?
         │                       │
   TouchLastSeen            CreateUser (MongoDB)
         │                       │
         └──────────┬────────────┘
                    ▼
           UpsertPresence (RAM)  ← seeds isSearching=false, isMatched=false
                    ▼
           GenerateToken (JWT)
           payload: { userId, deviceId, exp: 720h }
                    ▼
           Return: { token, expiresAt, user, presence }
```

**JWT Middleware:**
- Every protected route runs `AuthRequired()`
- Reads `Authorization: Bearer <token>` header
- Calls `utils.ParseToken()` → validates HMAC-SHA256 signature
- Injects `authUserID` and `authDeviceID` into Gin context
- **Stateless** — no session table; the token IS the session

---

### 2.2 Presence Engine

**File:** `repository/presence_repository.go`

**Data structure:**
```go
presenceStore {
    mu       sync.RWMutex          // RLock for reads, Lock for writes
    sessions map[string]*Presence  // userID(hex) → *Presence
}

Presence {
    UserID, DeviceID
    LastActiveAt   time.Time   // updated by heartbeat every ~20s
    IsSearching    bool
    IsMatched      bool
    CurrentMatchID *ObjectID
    ActiveChatID   *string
    TypingTo       *string
    UpdatedAt      time.Time
}
```

**Operations and lock types:**

| Operation | Lock | Triggered by |
|---|---|---|
| `FindPresenceByUserID` | `RLock` (shared) | GET /presence/:userId |
| `Heartbeat` | `Lock` (exclusive) | POST /presence/heartbeat every 20s |
| `UpdatePresence` | `Lock` | PATCH /presence |
| `SetMatched` | `Lock` | After match is made |
| `ClearMatchState` | `Lock` | Match ends or search cancelled |

**IsOnline logic:**
```go
func (p *Presence) IsOnline() bool {
    return time.Since(p.LastActiveAt) < 30*time.Second
}
// No polling needed — derived on demand from lastActiveAt timestamp
```

---

### 2.3 Matchmaking Engine (Long-Polling Queue)

**File:** `repository/match_repository.go`, `service/match_service.go`, `controller/match_controller.go`

**Three independent in-memory structures (never nested to prevent deadlock):**

```
matchQueue:      sync.Mutex  +  []string (FIFO userID queue)
waiterRegistry:  sync.Mutex  +  map[userID]chan MatchResult
activeMatches:   sync.RWMutex + map[matchID]*Match + map[userID]matchID
```

**Full long-polling sequence:**

```
User A: POST /matches/search
────────────────────────────────────────────────────────────
1. UpdatePresence(isSearching=true) in RAM       [write lock]
2. RegisterWaiter(userA) → creates chan (buf=1)  [write lock]
3. Enqueue(userA):
   - queue.Lock()
   - queue is empty → append userA → queue.Unlock()
   - return matched=false
4. HTTP handler blocks: select { case result := <-waitCh }
   [Goroutine parked here — uses 0 CPU while waiting]


User B: POST /matches/search
────────────────────────────────────────────────────────────
1. UpdatePresence(isSearching=true) in RAM       [write lock]
2. RegisterWaiter(userB) → creates chan (buf=1)  [write lock]
3. Enqueue(userB):
   - queue.Lock()
   - queue has [userA] → pop userA → queue.Unlock()
   - storeMatch(userB, userA):
       - activeMatches.Lock()
       - create Match{matchID, user1=userB, user2=userA}
       - store in matches + byUser maps
       - activeMatches.Unlock()
       - notifyWaiter(userA, {matchID, partnerID=userB})
         → sends to userA's buffered channel (non-blocking)
   - return matched=true, matchID, partnerID=userA
4. User B's handler: got instant match → 200 OK immediately
   User A's handler: channel fires → 200 OK immediately


Timeout path (no partner in 30s):
────────────────────────────────────────────────────────────
select {
    case <-time.After(30s):
        CancelSearch() → Dequeue + RemoveWaiter + ClearMatchState
        → 202 Accepted "still searching... call again"
    case <-ctx.Done():
        CancelSearch() → same cleanup
        → connection dropped silently
}
```

**Lock order guarantee (deadlock prevention):**
```
Rule: queue.mu is ALWAYS released before acquiring activeMatches.mu
      No two of {queue.mu, waiters.mu, activeMatches.mu} are ever held simultaneously
```

---

### 2.4 WebSocket Match Room System

**Files:** `repository/ws_room_repository.go`, `controller/ws_controller.go`

**Data structure:**
```go
roomStore {
    mu    sync.RWMutex
    rooms map[matchID]map[userID]*WSClient
}

WSClient {
    UserID, MatchID string
    conn  *websocket.Conn
    send  chan []byte   // buffered, size 64
}
```

**Connection lifecycle:**
```
Client: GET /ws/match/:matchId?token=<jwt>
    ↓
1. ParseToken(jwt)       → extract userID
2. GetMatchByUser(userID) → verify user owns this matchID [RLock]
3. wsUpgrader.Upgrade()  → HTTP → WebSocket TCP upgrade
4. JoinRoom(matchID, userID, conn) → register in RAM     [Lock]
5. go client.WritePump() → goroutine: drains send channel → TCP write
6. client.ReadPump(...)  → BLOCKS: reads TCP → dispatches to handler
    On disconnect:
        LeaveRoom(matchID, userID)  → close send chan, cleanup room [Lock]
        SendToClient(partner, PARTNER_DISCONNECTED)

Message dispatch (inside ReadPump):
    raw JSON → unmarshal type field → switch:
        SEND_FRIEND_REQUEST   → service.SendFriendRequest()
        ACCEPT_FRIEND_REQUEST → service.ResolveFriendRequest(accept=true)
        REJECT_FRIEND_REQUEST → service.ResolveFriendRequest(accept=false)
        unknown               → log + ignore
```

**Goroutine budget per active match:**
```
2 users × (1 WritePump goroutine + 1 ReadPump goroutine) = 4 goroutines/match
Memory: ~32 KB goroutine stacks + 2 × 512B send buffers ≈ 33 KB/match
```

---

### 2.5 Friend Request System

**Files:** `repository/friendship_repository.go`, `service/friend_service.go`, `controller/friend_controller.go`

**MongoDB collections:**

```
friend_requests {
    _id, matchId, fromId, toId
    status: "pending" | "accepted" | "rejected"
    createdAt, updatedAt
}
Indexes: (toId, status), (matchId, fromId, status)

friendships {
    _id, user1Id, user2Id, matchId, createdAt
}
Indexes: (user1Id), (user2Id)
```

**Send friend request flow:**
```
POST /friends/requests  { matchId }    ← HTTP fallback
WS:  SEND_FRIEND_REQUEST               ← primary path

service.SendFriendRequest(senderID, matchID):
    1. GetMatchByUser(senderID)           [RAM RLock]
       → verify match exists + sender belongs to it
    2. Determine partnerID from match struct
    3. CreateFriendRequest(MongoDB):
       a. Check no duplicate pending request for this matchId+fromId
       b. Check AreFriends() — no existing friendship
       c. Insert document
    4. GetPartner(matchID, senderID)      [RAM RLock]
       → find partner's WSClient if connected
    5. SendToClient(partner, FRIEND_REQUEST_RECEIVED) [non-blocking chan send]
```

**Accept/Reject flow:**
```
service.ResolveFriendRequest(resolverID, requestID, accept):
    1. GetFriendRequestByID (MongoDB)
    2. Validate: req.toId == resolverID (only receiver can resolve)
    3. Validate: req.status == "pending"
    4. Update status in MongoDB (accepted | rejected)
    5. If accepted: Insert Friendship document (MongoDB)
    6. GetPartner() → push WS event to original sender:
       accepted → FRIEND_REQUEST_ACCEPTED { requestId, friendshipId }
       rejected → FRIEND_REQUEST_REJECTED
```

---

### 2.6 MongoDB Index Strategy

```
users:
  - deviceId (unique)     → fast anonymous login lookup
  - isSearching           → legacy, not used by current RAM-based engine

friend_requests:
  - (toId, status)        → GET /friends/requests: "all pending for me"
  - (matchId, fromId, status) → duplicate guard on request creation

friendships:
  - user1Id               → half of "get my friends" query
  - user2Id               → other half of "get my friends" query

presence: (currently unused — presence is 100% in-memory)
  - userId (unique)
  - isSearching
  - lastActiveAt
```

---

### 2.7 Concurrency Limits Analysis

| Component | Concurrency Model | Practical Ceiling |
|---|---|---|
| HTTP connections | 1 goroutine/request | ~100,000 |
| Long-polling waiters | 1 parked goroutine/user | ~50,000 |
| Presence heartbeat engine | In-memory RWMutex + heartbeat updates | ~50,000+ users |
| Match queue scan | O(n) slice scan | ~10,000 (queue never large) |
| WebSocket rooms | 4 goroutines/match | ~10,000 matched users |
| MongoDB pool | 100 connections max | ~5,000 write-heavy users |

**Overall system ceiling: tens of thousands of concurrent users on a single instance depending on hardware configuration and websocket activity.**

---

### 2.8 Complete API Surface

#### Public
| Method | Path | Description |
|---|---|---|
| GET | `/health` | Uptime check |
| POST | `/api/v1/auth/anonymous` | Register or login |

#### WebSocket
| Method | Path | Description |
|---|---|---|
| GET | `/ws/match/:matchId?token=jwt` | Join active match room |

#### Protected (JWT required)
| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/users/me` | Get own profile |
| PATCH | `/api/v1/users/me` | Update own profile |
| GET | `/api/v1/users/:userId` | Get any user's profile |
| POST | `/api/v1/presence/heartbeat` | Keep-alive ping (~20s) |
| PATCH | `/api/v1/presence` | Update presence fields |
| GET | `/api/v1/presence/:userId` | Get user presence |
| POST | `/api/v1/matches/search` | Enter queue (long-poll) |
| DELETE | `/api/v1/matches/search` | Cancel search |
| GET | `/api/v1/matches/me` | Get current match (reconnect) |
| DELETE | `/api/v1/matches/me` | Leave match |
| POST | `/api/v1/friends/requests` | Send friend request (HTTP) |
| GET | `/api/v1/friends/requests` | List pending received requests |
| POST | `/api/v1/friends/requests/:id/accept` | Accept request |
| POST | `/api/v1/friends/requests/:id/reject` | Reject request |
| GET | `/api/v1/friends` | List all friendships |

#### WebSocket Events (Client → Server)
| Type | Payload | Description |
|---|---|---|
| `SEND_FRIEND_REQUEST` | — | Send request to partner |
| `ACCEPT_FRIEND_REQUEST` | `{ requestId }` | Accept received request |
| `REJECT_FRIEND_REQUEST` | `{ requestId }` | Reject received request |

#### WebSocket Events (Server → Client)
| Type | Payload | Description |
|---|---|---|
| `FRIEND_REQUEST_RECEIVED` | `{ requestId, fromId, matchId }` | Partner sent you a request |
| `FRIEND_REQUEST_ACCEPTED` | `{ requestId, friendshipId }` | Your request was accepted |
| `FRIEND_REQUEST_REJECTED` | `{ requestId }` | Your request was rejected |
| `PARTNER_DISCONNECTED` | — | Partner left the WS room |
| `FRIEND_REQUEST_SENT` | `{ requestId }` | Confirmation echo to sender |
| `ERROR` | `{ message }` | Operation failed |
