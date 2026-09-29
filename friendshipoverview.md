# 🤝 RanChat Friendship Subsystem — Comprehensive Technical Overview & Evaluation

**Evaluated Module:** [`internals/friendship`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship)  
**Evaluation Date:** September 29, 2026  
**Overall Architectural Rating:** **6.6 / 10**  

---

## 📊 Executive Summary & Scorecard

The RanChat Friendship subsystem provides persistent user connections and social networking features over MongoDB (`friends` and `friendrequests` collections). It enables users to send friend requests, accept or decline incoming requests, view pending requests, list established friendships, and remove friends.

The domain follows standard Layered Architecture / DDD principles (Controllers ➔ Services ➔ Repository ➔ MongoDB Collections) and is cleanly integrated into Gin router with `AuthMiddleware`.

However, the current implementation contains **critical Insecure Direct Object Reference (IDOR) authorization vulnerabilities, missing reciprocal relationship validation checks, un-indexed MongoDB collection scans, non-idiomatic Go error return tuples, and an unpopulated 0-byte DTO file**.

| Domain / Dimension | Score | Assessment & Observations |
| :--- | :---: | :--- |
| **API & UX Design** | **8.0 / 10** | Clean RESTful endpoint design (`/v1/friendship/requests`, `/v1/friendship/friends`), appropriate HTTP status codes (201, 200, 401, 400). |
| **Code Architecture & Layering** | **8.5 / 10** | Excellent separation of concerns (Controller ➔ Service ➔ Repository ➔ MongoDB Driver v2). Zero circular imports. |
| **Security & Authorization** | **4.5 / 10** | **Critical Vulnerability:** Missing resource ownership verification in `AcceptFriendRequest`, `DeclineFriendRequest`, and `RemoveFriend`. IDOR risks allow any authenticated user to accept/decline arbitrary requests or delete arbitrary friendships by ID. |
| **Domain Logic & Data Integrity** | **5.5 / 10** | Missing validation against self-friend requests, reverse pending requests, or existing active friendships. |
| **Database Performance & Indexing** | **6.0 / 10** | Lacks MongoDB compound indexes on `receiver_id`, `sender_id`, or `user_ids`. `Find` operations will perform full collection scans at scale. |
| **Code Hygiene & Go Idioms** | **6.0 / 10** | Non-idiomatic return parameter ordering in repository methods (`(error, Result)` instead of `(Result, error)`). Empty 0-byte file `dto/friend_request`. |

---

## 🔬 System Workflow Architecture

### 1. Send Friend Request Flow
```
[ POST /v1/friendship/requests ] (AuthMiddleware)
         │
         ▼
 1. Extract authenticated `userId` from Gin Context (senderID)
         │
         ▼
 2. Bind JSON body (`receiver_id`) into `models.FriendRequest`
         │
         ▼
 3. Overwrite `request.SenderID` with authenticated `senderID`
         │
         ▼
 4. Check `FriendRequestDB` for existing `(sender_id, receiver_id)`
         ├── Existing Request Found ──► Return HTTP 500 ("friend request already exists")
         └── No Existing Request   ──► Insert `models.FriendRequest` & Return HTTP 201 Created
```

---

### 2. Accept Friend Request Flow
```
[ POST /v1/friendship/requests/:requestId/accept ] (AuthMiddleware)
         │
         ▼
 1. Extract `requestId` from URL path parameter
         │
         ▼
 2. Query `FriendRequestDB` by `_id = requestId`
         │
 3. Create `models.Friend` document with `UserIDs = [SenderID, ReceiverID]`
         │
 4. Insert into `FriendDB` (`friends` collection)
         │
 5. Delete original request from `FriendRequestDB` (`friendrequests` collection)
         │
 6. Return HTTP 200 OK `{ message: "Friend request accepted", friend: ... }`
```

---

### 3. Decline & Remove Workflows
```
[ POST /v1/friendship/requests/:requestId/decline ] ──► Delete from `FriendRequestDB` where `_id = requestId` ──► Return HTTP 200 OK
[ DELETE /v1/friendship/friends/:friendshipID ]    ──► Delete from `FriendDB` where `_id = friendshipID`    ──► Return HTTP 200 OK
```

---

## 🚨 Detailed Technical Audit & Identified Vulnerabilities

### 🔴 1. Insecure Direct Object Reference (IDOR) & Broken Authorization
* **Files:** [`friendship_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L63-L127) & [`friendship_services.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/services/friendship_services.go#L24-L51) & [`friendship_controller.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/controllers/friendship_controller.go#L80-L171)

#### Issues:
1. **Accepting Requests:**
   In [`friendship_controller.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/controllers/friendship_controller.go#L100-L104), `userID` (authenticated user) is passed into `fc.Service.AcceptFriendRequest(ctx, requestID, userID)`.
   However, in [`friendship_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L63-L71), the repository method signature is:
   ```go
   func (f *MongoFriendshipRepository) AcceptFriendRequest(ctx context.Context, requestId string) (error, *models.Friend)
   ```
   The `userID` is completely dropped! The query checks only `bson.M{"_id": rId}`.
   **Exploit:** User A sends a friend request to User B. User C (an attacker) passes `requestId` to `POST /v1/friendship/requests/<requestId>/accept` with their own JWT token. The request succeeds! User A and User B become friends without User B's consent.

2. **Declining Requests:**
   [`DeclineFriendRequest`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L98-L112) deletes `_id = requestId` without verifying whether the authenticated user is either `receiver_id` or `sender_id`.
   **Exploit:** Any user can delete any pending friend request across the system if they know the ObjectID hex string.

3. **Removing Friends:**
   [`RemoveFriend`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L114-L127) deletes `_id = friendId` from `FriendDB` without verifying if the authenticated user's ID exists inside `user_ids`.
   **Exploit:** Any authenticated user can delete any friendship record between any two users in the database by ID.

---

### 🔴 2. Missing Domain Rules & Reciprocal Validation

* **Files:** [`friendship_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L34-L62)

```go
func (f *MongoFriendshipRepository) SendFriendRequest(ctx context.Context, requestObj models.FriendRequest) error {
    filter := bson.M{
        "sender_id":   requestObj.SenderID,
        "receiver_id": requestObj.ReceiverID,
    }
    ...
}
```

#### Deficiencies:
1. **Self-Friend Request:** No check ensuring `SenderID != ReceiverID`. A user can send a friend request to themselves.
2. **Reverse Pending Request:** Checks `sender_id = A, receiver_id = B`, but does NOT check `sender_id = B, receiver_id = A`. If B has already sent A a request, A can send a duplicate pending request back to B instead of auto-accepting.
3. **Already Friends Check:** Does not query `FriendDB` (`friends` collection) to check if `[A, B]` are already friends before creating a new friend request.
4. **Resending Declined Requests:** If a request is declined (deleted from `friendrequests`), sending another request creates a new document. If a request is pending, attempting to re-send returns a generic error string rather than a typed sentinel error.

---

### 🟡 3. Un-indexed MongoDB Collections & Query Performance

* **File:** [`friendship_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L129-L184)

```go
// GetAllFriends
filter := bson.M{ "user_ids": uID }
cursor, err := f.FriendDB.Find(ctx, filter)

// GetAllFriendRequest
filter := bson.M{ "receiver_id": uID }
cursor, err := f.FriendRequestDB.Find(ctx, filter)
```

* **Issue:** Neither `friends` nor `friendrequests` collections have indexes created during startup (`cmd/main.go` or repository initialization).
* **Impact:**
  - `GetAllFriends` requires a full scan of the `friends` collection matching items in `user_ids`.
  - `GetAllFriendRequest` requires a full scan of the `friendrequests` collection filtering by `receiver_id`.
  - As database document count grows into thousands/millions, response times for listing friends and requests will degrade significantly.

---

### 🟡 4. Non-Standard Go Error Return Ordering

* **File:** [`friendship_repository.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/repository/friendship_repository.go#L15-L19)

```go
type FriendshipRepository interface {
    SendFriendRequest(context.Context, models.FriendRequest) error
    AcceptFriendRequest(context.Context, string) (error, *models.Friend) // ❌ Non-idiomatic
    DeclineFriendRequest(context.Context, string) error
    RemoveFriend(context.Context, string) error
    GetAllFriendRequest(context.Context, string) (error, []models.FriendRequest) // ❌ Non-idiomatic
    GetAllFriends(context.Context, string) (error, []models.Friend) // ❌ Non-idiomatic
}
```

* **Violation:** Standard Go convention dictates that `error` is always the **last** return value in multi-value return signatures (`(T, error)`). Placing `error` as the first return value breaks common IDE auto-complete patterns and Go linter conventions.

---

### 🔵 5. Empty DTO File & Request Binding Issues

* **File:** [`internals/friendship/dto/friend_request`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/dto/friend_request) (0 bytes)

* **Issue:** The `dto/friend_request` file is completely empty (0 bytes).
* **Consequence:** In [`friendship_controller.go`](file:///c:/Users/mayank/Desktop/goprojects/ranchat/internals/friendship/controllers/friendship_controller.go#L54), `ctx.ShouldBindJSON(&request)` binds directly to `models.FriendRequest`. Clients can supply arbitrary BSON/JSON fields (`_id`, `status`, `created_at`) directly in the HTTP POST body. Although `SenderID` is overwritten, `Status` and timestamps are left unvalidated.

---

## 📁 Repository Code Anatomy & Files Overview

```text
internals/friendship/
├── controllers/
│   └── friendship_controller.go  # Gin HTTP Handlers for Friend Request & Friend list endpoints
├── dto/
│   └── friend_request            # ⚠️ 0-byte placeholder file (needs proper DTO struct definition)
├── models/
│   ├── friend.go                 # Friend domain entity struct (ID, UserIDs []ObjectID, CreatedAt)
│   └── friend_request.go         # FriendRequest domain entity struct (SenderID, ReceiverID, Status, Timestamps)
├── repository/
│   └── friendship_repository.go  # Mongo DB v2 driver operations on `friends` and `friendrequests`
├── routes/
│   └── friendship_routes.go      # Gin engine group `/v1/friendship` registered with `AuthMiddleware`
└── services/
    └── friendship_services.go    # Friendship domain service delegating to FriendshipRepository
```

---

## 🌐 Complete API Endpoints Reference

All endpoints are prefixed with `/v1/friendship` and require a valid Bearer JWT token in the `Authorization` header (`AuthMiddleware`).

| Endpoint | Method | Auth | Path / Query Parameters | Request Body | Success Response | Error Responses |
| :--- | :---: | :---: | :--- | :--- | :--- | :--- |
| `/v1/friendship/requests` | `POST` | 🔐 | None | `{"receiver_id": "hex_id"}` | `201 Created`<br>`{"message": "Friend request sent successfully"}` | `400 Bad Request` (Invalid JSON)<br>`401 Unauthorized`<br>`500 Internal Error` |
| `/v1/friendship/requests/:requestId/accept` | `POST` | 🔐 | `requestId`: Hex ObjectID | None | `200 OK`<br>`{"message": "Friend request accepted", "friend": {...}}` | `400 Bad Request`<br>`401 Unauthorized` |
| `/v1/friendship/requests/:requestId/decline` | `POST` | 🔐 | `requestId`: Hex ObjectID | None | `200 OK`<br>`{"message": "Friend request declined"}` | `400 Bad Request`<br>`401 Unauthorized` |
| `/v1/friendship/requests` | `GET` | 🔐 | None | None | `200 OK`<br>`{"requests": [...]}` | `401 Unauthorized`<br>`500 Internal Error` |
| `/v1/friendship/friends` | `GET` | 🔐 | None | None | `200 OK`<br>`{"friends": [...]}` | `401 Unauthorized`<br>`500 Internal Error` |
| `/v1/friendship/friends/:friendshipID` | `DELETE` | 🔐 | `friendshipID`: Hex ObjectID | None | `200 OK`<br>`{"message": "Friend removed successfully"}` | `400 Bad Request`<br>`401 Unauthorized` |

---

## 🛠️ Step-by-Step Remediation Plan

### Phase 1: Security & IDOR Authorization Fixes
1. **Repository Authorization Signatures:**
   Update repository methods to accept `authenticatedUserID`:
   ```go
   AcceptFriendRequest(ctx context.Context, requestID string, authenticatedUserID string) (*models.Friend, error)
   DeclineFriendRequest(ctx context.Context, requestID string, authenticatedUserID string) error
   RemoveFriend(ctx context.Context, friendshipID string, authenticatedUserID string) error
   ```
2. **Resource Ownership Filter Clauses:**
   - In `AcceptFriendRequest`: Query `bson.M{"_id": rId, "receiver_id": uID}` to guarantee only the intended recipient can accept the request.
   - In `DeclineFriendRequest`: Query `bson.M{"_id": rId, "$or": []bson.M{{"receiver_id": uID}, {"sender_id": uID}}}`.
   - In `RemoveFriend`: Query `bson.M{"_id": fId, "user_ids": uID}` so only a member of the friendship can delete it.

---

### Phase 2: Domain Logic & Reciprocal Validation
1. **Self-Request Guard:**
   In `SendFriendRequest`, reject requests where `SenderID == ReceiverID`.
2. **Check Reciprocal Requests & Existing Friendships:**
   - Query `FriendRequestDB` for `(sender_id = B AND receiver_id = A)`.
   - Query `FriendDB` for `user_ids: { $all: [A, B] }`.

---

### Phase 3: MongoDB Indexing & Go Idiom Cleanup
1. **Add Indexing in MongoDB Setup:**
   Create compound unique index on `friendrequests`: `{ sender_id: 1, receiver_id: 1 }` with `{ unique: true }`.
   Create multikey index on `friends`: `{ user_ids: 1 }`.
2. **Refactor Function Return Orders:**
   Standardize return values across `FriendshipRepository` and `FriendshipService` to `(Result, error)`.
3. **Populate Request DTO:**
   Define `SendFriendRequestDTO` in `internals/friendship/dto/send_friend_request.go` containing explicit `ReceiverID string` validation tags (`binding:"required,len=24"`).

---

## 📈 Metric & Health Comparison

| Metric / Dimension | Current Implementation | Target Post-Remediation |
| :--- | :---: | :---: |
| **Authorization Security (IDOR)** | 🔴 4.5 / 10 | 🟢 9.5 / 10 |
| **Data Integrity & Validation** | 🟡 5.5 / 10 | 🟢 9.0 / 10 |
| **Query Performance (Indexes)** | 🟡 6.0 / 10 | 🟢 9.5 / 10 |
| **Go Code Conventions** | 🟡 6.0 / 10 | 🟢 9.5 / 10 |
| **Overall Module Rating** | **6.6 / 10** | **9.4 / 10** |
