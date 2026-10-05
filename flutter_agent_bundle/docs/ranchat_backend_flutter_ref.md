# 📘 RanChat Backend & Flutter Front-End Technical Reference Manual

**Document Version:** 1.0.0  
**Target Platform:** Flutter (iOS, Android, Web)  
**Backend Technology Stack:** Go 1.26 (Gin Framework), Redis v9 (Presence Engine), MongoDB Driver v2, Gorilla WebSocket v1.5, JWT Auth  
**Last Updated:** September 30, 2026  

---

## 📑 Table of Contents

1. [System Architecture & Core Concepts](#1-system-architecture--core-concepts)
2. [Complete REST API Specification](#2-complete-rest-api-specification)
   - [Authentication Subsystem (`/v1/auth`)](#authentication-subsystem-v1auth)
   - [Presence Subsystem (`/v1/presence`)](#presence-subsystem-v1presence)
   - [Matchmaking Engine (`/v1/matching`)](#matchmaking-engine-v1matching)
   - [Friendship Subsystem (`/v1/friendship`)](#friendship-subsystem-v1friendship)
3. [WebSocket Protocol & Real-Time Events (`/v1/ws`)](#3-websocket-protocol--real-time-events-v1ws)
4. [Sequence & State Machine Diagrams](#4-sequence--state-machine-diagrams)
5. [Dart Data Models & Code Specs](#5-dart-data-models--code-specs)
6. [Backend Audit Notes & Client Safeguards](#6-backend-audit-notes--client-safeguards)
7. [Production Flutter Implementation Snippets](#7-production-flutter-implementation-snippets)

---

## 🏗️ 1. System Architecture & Core Concepts

RanChat is an Omegle-like random pairing platform that connects users based on reciprocal interest tags and gender preferences. The backend consists of several bounded domain contexts:

```
┌────────────────────────────────────────────────────────────────────────┐
│                        RanChat Backend Services                        │
├───────────────┬────────────────┬─────────────────┬─────────────────────┤
│  Auth (Mongo) │ Presence (Redis)│ Matchmaker (RAM)│ WebSocket Hub (RAM) │
└───────▲───────┴───────▲────────┴────────▲────────┴──────────▲──────────┘
        │               │                 │                    │
        │ HTTP REST     │ 15s Heartbeat   │ 30s Long-Poll      │ WebSocket Connection
        │               │                 │                    │
┌───────┴───────────────┴─────────────────┴────────────────────┴──────────┐
│                           Flutter Mobile Client                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Key Architectural Behaviors to Respect in Flutter:
1. **Asynchronous Onboarding:** `POST /v1/auth/signup` writes to MongoDB and immediately returns a JWT. Redis presence initialization happens asynchronously via event dispatching.
2. **Redis Heartbeat Renewal:** Presence keys in Redis (`presence:<userID>`) expire if not updated within **20 seconds**. The Flutter app MUST issue a heartbeat every **15 seconds** while online.
3. **HTTP Long-Polling Matchmaking:** `POST /v1/matching/start` holds the HTTP request connection open for up to **30 seconds**. The Dio HTTP client `receiveTimeout` MUST be configured to **35 seconds**.
4. **WebSocket Connection Hub:** Upgraded connection at `/v1/ws` receives server-pushed notifications (e.g. `friend_request_received`).

---

## 🌐 2. Complete REST API Specification

All protected endpoints require the HTTP header:
`Authorization: Bearer <JWT_TOKEN>`

### Authentication Subsystem (`/v1/auth`)

#### 1. User Registration / Signup
- **Method:** `POST`
- **Path:** `/v1/auth/signup`
- **Access:** Public
- **Request Headers:** `Content-Type: application/json`
- **Request Body Payload:**
  ```json
  {
    "name": "Jane Doe",
    "age": 23,
    "gender": "female",
    "bio": "Software engineer & gamer",
    "interest": ["coding", "gaming", "music"],
    "deviceId": "d6a4f91b-7c81-4b2a-89e4-811c750b3294"
  }
  ```
- **Field Constraints:**
  - `gender`: `"male"`, `"female"`, or `"other"`
  - `interest`: Non-empty array of strings
  - `deviceId`: Non-empty string (UUID v4 recommended)
- **Response `201 Created`:**
  ```json
  {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6IjY2ZmE1NDEyMmIxMWQ4YzExZTc0YTgxMiIsImV4cCI6MTcyNzY5NjAwMH0..."
  }
  ```
- **Response `400 Bad Request`:**
  ```json
  {
    "error": "Invalid request body payload"
  }
  ```

#### 2. Get User Profile
- **Method:** `GET`
- **Path:** `/v1/auth/user`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "user": {
      "user_id": "66fa54122b11d8c11e74a812",
      "name": "Jane Doe",
      "age": 23,
      "gender": "female",
      "bio": "Software engineer & gamer",
      "interest": ["coding", "gaming", "music"],
      "deviceId": "d6a4f91b-7c81-4b2a-89e4-811c750b3294",
      "premiumTill": "0001-01-01T00:00:00Z",
      "createdAt": "2026-09-29T10:30:00Z",
      "updatedAt": "2026-09-29T10:30:00Z"
    }
  }
  ```
- **Response `401 Unauthorized`:**
  ```json
  {
    "error": "Missing authorization token header"
  }
  ```

---

### Presence Subsystem (`/v1/presence`)

#### 1. Initialize Presence
- **Method:** `POST`
- **Path:** `/v1/presence`
- **Access:** Protected (JWT required)
- **Request Body Payload:**
  ```json
  {
    "deviceId": "d6a4f91b-7c81-4b2a-89e4-811c750b3294"
  }
  ```
- **Response `201 Created`:**
  ```json
  {
    "message": "presence created"
  }
  ```

#### 2. Heartbeat Ping
- **Method:** `POST`
- **Path:** `/v1/presence/heartbeat`
- **Access:** Protected (JWT required)
- **Request Body:** Empty `{}` or no body
- **Execution Interval:** Every **15 seconds** via periodic timer
- **Response `200 OK`:**
  ```json
  {
    "message": "heartbeat recorded"
  }
  ```

#### 3. Update Typing / Active Chat State
- **Method:** `PATCH`
- **Path:** `/v1/presence/update`
- **Access:** Protected (JWT required)
- **Request Body Payload:**
  ```json
  {
    "activeChatId": "66fa54992b11d8c11e74a899",
    "typingTo": "66fa54122b11d8c11e74a813"
  }
  ```
- **Response `200 OK`:**
  ```json
  {
    "message": "presence updated"
  }
  ```

---

### Matchmaking Engine (`/v1/matching`)

#### Start Reciprocal Match Search
- **Method:** `POST`
- **Path:** `/v1/matching/start`
- **Access:** Protected (JWT required)
- **Request Body Payload:**
  ```json
  {
    "interest": "coding",
    "gender": "female",
    "gender_pref": "male"
  }
  ```
- **Matching Algorithm Criteria (Reciprocal Requirement):**
  - `candidate.interest == request.interest`
  - `candidate.gender == request.gender_pref`
  - `candidate.gender_pref == request.gender`
- **Response `200 OK` (Immediate or Within 30s - Match Found):**
  ```json
  {
    "status": "matched",
    "match": {
      "match_id": "66fa54992b11d8c11e74a899",
      "user_ids": [
        "66fa54122b11d8c11e74a812",
        "66fa54122b11d8c11e74a813"
      ],
      "created_at": "2026-09-29T10:30:00Z"
    }
  }
  ```
- **Response `200 OK` (After 30s Timeout - No Match Found):**
  ```json
  {
    "status": "retry",
    "message": "No match found within timeout, please retry"
  }
  ```
- **Client Handling Strategy:** If response status is `"retry"`, check if user is still on the Radar Searching screen. If yes, immediately dispatch another `POST /v1/matching/start` request.

---

### Friendship Subsystem (`/v1/friendship`)

#### 1. Send Friend Request
- **Method:** `POST`
- **Path:** `/v1/friendship/requests`
- **Access:** Protected (JWT required)
- **Request Body Payload:**
  ```json
  {
    "receiver_id": "66fa54122b11d8c11e74a813"
  }
  ```
- **Response `201 Created`:**
  ```json
  {
    "message": "friend request sent",
    "request": {
      "id": "66fa55112b11d8c11e74a900",
      "sender_id": "66fa54122b11d8c11e74a812",
      "receiver_id": "66fa54122b11d8c11e74a813",
      "status": "pending",
      "created_at": "2026-09-29T10:32:00Z"
    }
  }
  ```

#### 2. Get Pending Friend Requests
- **Method:** `GET`
- **Path:** `/v1/friendship/requests`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "requests": [
      {
        "id": "66fa55112b11d8c11e74a900",
        "sender_id": "66fa54122b11d8c11e74a813",
        "receiver_id": "66fa54122b11d8c11e74a812",
        "status": "pending",
        "created_at": "2026-09-29T10:32:00Z"
      }
    ]
  }
  ```

#### 3. Accept Friend Request
- **Method:** `POST`
- **Path:** `/v1/friendship/requests/:requestId/accept`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "message": "Friend request accepted",
    "friend": {
      "id": "66fa56002b11d8c11e74a999",
      "user_ids": [
        "66fa54122b11d8c11e74a813",
        "66fa54122b11d8c11e74a812"
      ],
      "created_at": "2026-09-29T10:35:00Z"
    }
  }
  ```

#### 4. Decline Friend Request
- **Method:** `POST`
- **Path:** `/v1/friendship/requests/:requestId/decline`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "message": "friend request declined"
  }
  ```

#### 5. Get Friends List
- **Method:** `GET`
- **Path:** `/v1/friendship/friends`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "friends": [
      {
        "id": "66fa56002b11d8c11e74a999",
        "user_ids": [
          "66fa54122b11d8c11e74a813",
          "66fa54122b11d8c11e74a812"
        ],
        "created_at": "2026-09-29T10:35:00Z"
      }
    ]
  }
  ```

#### 6. Remove Friend
- **Method:** `DELETE`
- **Path:** `/v1/friendship/friends/:friendshipID`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "message": "friend removed"
  }
  ```

---

### 💬 1-on-1 Chat Subsystem (`/v1/chat`)

#### 1. Send Message
- **Method:** `POST`
- **Path:** `/v1/chat/messages`
- **Access:** Protected (JWT required)
- **Request Body Payload:**
  ```json
  {
    "receiver_id": "66fa54122b11d8c11e74a813",
    "content": "Hey! How are you doing?",
    "message_type": "text"
  }
  ```
- **Response `201 Created`:**
  ```json
  {
    "message": {
      "id": "66fa57772b11d8c11e74a999",
      "conversation_id": "66fa54122b11d8c11e74a812_66fa54122b11d8c11e74a813",
      "sender_id": "66fa54122b11d8c11e74a812",
      "receiver_id": "66fa54122b11d8c11e74a813",
      "content": "Hey! How are you doing?",
      "message_type": "text",
      "status": "sent",
      "created_at": "2026-10-01T00:30:00Z"
    }
  }
  ```

#### 2. Get Chat History
- **Method:** `GET`
- **Path:** `/v1/chat/messages/:receiver_id?limit=50`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "messages": [
      {
        "id": "66fa57772b11d8c11e74a999",
        "conversation_id": "66fa54122b11d8c11e74a812_66fa54122b11d8c11e74a813",
        "sender_id": "66fa54122b11d8c11e74a812",
        "receiver_id": "66fa54122b11d8c11e74a813",
        "content": "Hey! How are you doing?",
        "message_type": "text",
        "status": "sent",
        "created_at": "2026-10-01T00:30:00Z"
      }
    ]
  }
  ```

#### 3. Mark Messages as Read
- **Method:** `PATCH`
- **Path:** `/v1/chat/messages/:receiver_id/read`
- **Access:** Protected (JWT required)
- **Response `200 OK`:**
  ```json
  {
    "message": "messages marked as read"
  }
  ```

---

## ⚡ 3. WebSocket Protocol & Real-Time Events (`/v1/ws`)

### Connection Handshake
- **Protocol:** WebSocket (`ws://` or `wss://`)
- **Endpoint:** `/v1/ws`
- **Authentication:** Pass Bearer token via query param `ws://localhost:8080/v1/ws?token=<JWT_TOKEN>` or HTTP upgrade header.

### Inbound Server Events (Server -> Client)

#### 1. Friend Request Event
```json
{
  "event": "friend_request_received",
  "data": {
    "request_id": "66fa55112b11d8c11e74a900",
    "sender_id": "66fa54122b11d8c11e74a813",
    "message": "You received a new friend request!"
  }
}
```

#### 2. Real-Time Chat Message Event
```json
{
  "event": "new_message",
  "data": {
    "id": "66fa57772b11d8c11e74a999",
    "conversation_id": "66fa54122b11d8c11e74a812_66fa54122b11d8c11e74a813",
    "sender_id": "66fa54122b11d8c11e74a812",
    "receiver_id": "66fa54122b11d8c11e74a813",
    "content": "Hey! How are you doing?",
    "message_type": "text",
    "status": "sent",
    "created_at": "2026-10-01T00:30:00Z"
  }
}
```

---

## 📊 4. Sequence & State Machine Diagrams

### Diagram A: Asynchronous User Onboarding Flow
```
[ Flutter App ]                           [ Gin Auth Controller ]              [ MongoDB ]         [ Redis Presence ]
      │                                             │                              │                   │
      │ ─── 1. POST /v1/auth/signup ──────────────► │                              │                   │
      │                                             │ ─── 2. Insert User ────────► │                   │
      │                                             │ ◄── 3. Success (UserID) ───  │                   │
      │                                             │                                                  │
      │ ◄── 4. 201 Created { token } ────────────── │ ─── (Asynchronous Event) ────────────────────────►│
      │                                                                                Create `presence:<UserID>`
```

### Diagram B: Periodic Presence Heartbeat Loop (15s Interval)
```
[ Flutter Presence Timer ]               [ Gin Presence Controller ]            [ Redis Presence Repository ]
            │                                         │                                      │
            │ ─── POST /v1/presence/heartbeat ──────► │                                      │
            │     (Headers: Bearer JWT)               │ ─── GetPresence("presence:<ID>") ──► │
            │                                         │ ◄── Return Presence JSON ──────────  │
            │                                         │                                      │
            │                                         │ ─── Set TTL = 20s ─────────────────► │
            │ ◄── 200 OK {"message":"recorded"} ──────│                                      │
```

### Diagram C: 30-Second HTTP Long-Polling Matchmaking Loop
```
[ Flutter Matchmaking Screen ]            [ Matchmaking Controller ]             [ Matchmaking Memory Pool ]
            │                                         │                                      │
            │ ─── POST /v1/matching/start ──────────► │                                      │
            │     { interest, gender, gender_pref }   │ ─── MatchAndEnqueue() ─────────────► │
            │                                         │                                      │
            │                                  ┌──────┴──────────────────────────────────────┐
            │                                  ▼                                             ▼
            │                         [ Match Found Immediately ]                   [ Waiting Pool (Select 30s) ]
            │                                  │                                             │
            │                                  │                                     ┌───────┴──────┐
            │                                  │                                     ▼              ▼
            │                                  │                             [ Partner Arrived]  [ 30s Timeout ]
            │                                  │                                     │              │
            │ ◄── 200 OK { status: "matched" } ┴─────────────────────────────────────┘              │
            │                                                                                       │
            │ ◄── 200 OK { status: "retry" } ───────────────────────────────────────────────────────┘
            │          │
            │          └── If user still searching, auto-dispatch POST /v1/matching/start again!
```

---

## 💻 5. Dart Data Models & Code Specs

### `UserModel` (`lib/features/auth/data/models/user_model.dart`)
```dart
import 'package:freezed_annotation/freezed_annotation.dart';

part 'user_model.freezed.dart';
part 'user_model.g.dart';

@freezed
class UserModel with _$UserModel {
  const factory UserModel({
    @JsonKey(name: 'user_id') required String userId,
    required String name,
    required int age,
    required String gender,
    required String bio,
    required List<String> interest,
    required String deviceId,
    DateTime? premiumTill,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) = _UserModel;

  factory UserModel.fromJson(Map<String, dynamic> json) => _$UserModelFromJson(json);
}
```

### `MatchModel` (`lib/features/matchmaking/data/models/match_model.dart`)
```dart
class MatchModel {
  final String matchId;
  final List<String> userIds;
  final DateTime createdAt;

  MatchModel({
    required this.matchId,
    required this.userIds,
    required this.createdAt,
  });

  factory MatchModel.fromJson(Map<String, dynamic> json) {
    return MatchModel(
      matchId: json['match_id'] ?? '',
      userIds: List<String>.from(json['user_ids'] ?? []),
      createdAt: DateTime.parse(json['created_at'] ?? DateTime.now().toIso8601String()),
    );
  }
}
```

---

## ⚠️ 6. Backend Audit Notes & Client Safeguards

1. **Long-Polling Timeout Configuration:**
   - Dio `receiveTimeout` MUST be set to **at least 35 seconds** (`Duration(seconds: 35)`). Setting it lower (e.g. 10s) will cause Dio to throw `DioExceptionType.receiveTimeout` prematurely while the backend server is still waiting for a match.
2. **Duplicate Match Mitigation:**
   - In rare high-concurrency race conditions, a user might receive a match notification right as their previous HTTP call times out. The Flutter state machine MUST verify if `state is Matched` before handling incoming responses, ignoring stale timeout frames.
3. **Friendship IDOR Workaround:**
   - Keep local copy of current `userId`. When rendering friend request options, verify on client UI that `request.receiverId == currentUserId` before displaying "Accept" / "Decline" buttons.

---

## 🚀 7. Production Flutter Implementation Snippets

### A. Network Client & Auth Interceptor (`lib/core/network/api_client.dart`)
```dart
import 'package:dio/dio.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class ApiClient {
  late final Dio dio;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  ApiClient({required String baseUrl}) {
    dio = Dio(
      BaseOptions(
        baseUrl: baseUrl,
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 35), // Critical for 30s long-polling!
        headers: {'Content-Type': 'application/json'},
      ),
    );

    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _storage.read(key: 'jwt_token');
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          return handler.next(options);
        },
      ),
    );
  }
}
```

### B. Periodic Heartbeat Engine (`lib/features/presence/presentation/presence_heartbeat_controller.dart`)
```dart
import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/network/api_client.dart';

final presenceHeartbeatProvider = Provider<PresenceHeartbeatController>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return PresenceHeartbeatController(apiClient: apiClient);
});

class PresenceHeartbeatController {
  final ApiClient apiClient;
  Timer? _timer;

  PresenceHeartbeatController({required this.apiClient});

  void startHeartbeat() {
    _timer?.cancel();
    // Execute heartbeat every 15 seconds to keep Redis TTL alive (20s expiration)
    _timer = Timer.periodic(const Duration(seconds: 15), (_) async {
      try {
        await apiClient.dio.post('/v1/presence/heartbeat');
      } catch (e) {
        // Silently log or handle network retry
      }
    });
  }

  void stopHeartbeat() {
    _timer?.cancel();
  }
}
```

---

> [!NOTE]
> Maintain this reference file alongside Flutter codebase development. All payload structures match backend models defined in `ranchat/internals`.
