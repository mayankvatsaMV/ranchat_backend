---
name: flutter-ranchat-frontend
description: Complete step-by-step instructions for an AI agent to build a production-grade Flutter front-end (iOS/Android/Web) for the RanChat application, including Clean Architecture, Riverpod state management, Dio HTTP long-polling, gorilla WebSocket connection hub, Redis heartbeat loops, and modern glassmorphic UI design.
---

# 📱 RanChat Flutter Front-End Development Skill

This skill guides an AI coding agent through building a complete, high-performance Flutter mobile and web client for **RanChat** — an Omegle-style random text/video chat platform with interest/gender matching, real-time presence tracking, persistent friendships, and WebSocket notifications.

---

## 📖 Master Index & Core Specs

Before beginning code generation or feature implementation, read the single master entry document:
👉 **[FLUTTER_AGENT_MASTER.md](file:///c:/Users/mayank/Desktop/goprojects/ranchat/FLUTTER_AGENT_MASTER.md)**

This single master index internally links and guides you through the 3 primary technical specification files:
1. 📄 **[PROJECT_OVERVIEW.md](file:///c:/Users/mayank/Desktop/goprojects/ranchat/PROJECT_OVERVIEW.md)** — Architecture, Auth, Redis Presence, MongoDB models, API reference.
2. 📄 **[matchmakingoverview.md](file:///c:/Users/mayank/Desktop/goprojects/ranchat/matchmakingoverview.md)** — Reciprocal matching algorithm, 30s HTTP long-polling, concurrency.
3. 📄 **[friendshipoverview.md](file:///c:/Users/mayank/Desktop/goprojects/ranchat/friendshipoverview.md)** — Friend requests, active friendships, WebSocket notifications.
4. 📄 **[ranchat_backend_flutter_ref.md](file:///c:/Users/mayank/Desktop/goprojects/ranchat/docs/ranchat_backend_flutter_ref.md)** — Deep-dive Flutter Dart data models & Dio network interceptors.

---

## 🏗️ Recommended Flutter Architecture Blueprint

The Flutter project **MUST** follow Clean Architecture with feature-first modularization:

```text
lib/
├── main.dart                             # App entry point, ProviderScope, router setup
├── core/
│   ├── config/
│   │   ├── env_config.dart               # API URLs, WS URLs, timeouts
│   │   └── theme.dart                    # Dark premium palette, typography, glassmorphism
│   ├── network/
│   │   ├── api_client.dart               # Dio instance with JWT AuthInterceptor & logging
│   │   ├── api_endpoints.dart            # Constant endpoint URIs (/v1/auth, /v1/matching, etc.)
│   │   └── ws_client.dart                # WebSocket channel client with reconnect & event stream
│   ├── storage/
│   │   └── secure_storage.dart           # FlutterSecureStorage for JWT & DeviceID
│   └── utils/
│       ├── device_info.dart              # Unique Device ID generator/fetcher
│       └── result.dart                   # Either/Result type for functional error handling
├── features/
│   ├── auth/
│   │   ├── data/
│   │   │   ├── models/user_model.dart    # User JSON serialization
│   │   │   └── repositories/auth_repo.dart
│   │   ├── presentation/
│   │   │   ├── providers/auth_provider.dart
│   │   │   └── screens/signup_screen.dart
│   ├── presence/
│   │   ├── data/presence_repo.dart
│   │   └── presentation/presence_heartbeat_controller.dart # 15s Timer heartbeat loop
│   ├── matchmaking/
│   │   ├── data/models/match_model.dart
│   │   ├── data/repositories/matchmaking_repo.dart
│   │   └── presentation/
│   │       ├── providers/matchmaking_provider.dart
│   │       └── screens/radar_matching_screen.dart # Ripple animation & filter controls
│   ├── chat/
│   │   ├── data/models/chat_message.dart
│   │   └── presentation/
│   │       ├── providers/chat_provider.dart
│   │       └── screens/active_chat_screen.dart   # Live text chat & WebRTC video box
│   └── friendship/
│       ├── data/models/friend_model.dart
│       ├── data/repositories/friendship_repo.dart
│       └── presentation/
│           ├── providers/friendship_provider.dart
│           └── screens/friends_list_screen.dart  # Friends, Pending Request badges
```

---

## 🎯 Step-by-Step Development Execution Plan

### Phase 1: Core Setup & Dependency Configuration
- Initialize a Flutter project (`flutter create --org com.ranchat ranchat_frontend`).
- Dependencies to include:
  - `flutter_riverpod` or `hooks_riverpod` for reactive state management.
  - `dio` for REST HTTP calls (configured with 35s receive timeout for 30s long-polling).
  - `web_socket_channel` for bidirectional persistent WebSocket communication.
  - `flutter_secure_storage` to persist `jwt_token` and `device_id`.
  - `uuid` for generating unique fallback device IDs.
  - `go_router` for declarative navigation.
  - `google_fonts` & `flutter_spinkit` / `lottie` for premium UI visuals.

### Phase 2: Authentication & Token Management
1. **Device ID Generation**: Check `flutter_secure_storage`. If absent, generate a UUID v4 string and save it.
2. **Signup Request**: Send `POST /v1/auth/signup` with `name`, `age`, `gender`, `bio`, `interest` array, and `deviceId`.
3. **Token Persistence**: Store the returned JWT token securely.
4. **Auth Interceptor**: Attach `Authorization: Bearer <token>` automatically on all subsequent Dio requests.

### Phase 3: Redis Presence & Background Heartbeat Engine
1. **Initialize Presence**: Call `POST /v1/presence` with `{ "deviceId": "..." }` right after user signup/login.
2. **Periodic Heartbeat Loop**:
   - Start a periodic Timer (`Timer.periodic(const Duration(seconds: 15), ...)`) to call `POST /v1/presence/heartbeat`.
   - Redis presence TTL is 20s; firing every 15s guarantees uninterrupted online status.
   - Pause or resume timer based on app lifecycle state (`AppLifecycleState.resumed` vs `paused`).

### Phase 4: Long-Polling Matchmaking System
1. **State Machine**: Implement states: `Idle`, `Searching`, `Matched(Match match)`, `Error(String msg)`, `TimeoutRetry`.
2. **Start Matching Action**:
   - Send `POST /v1/matching/start` with payload `{ "interest": "...", "gender": "...", "gender_pref": "..." }`.
   - Dio `receiveTimeout` **MUST** be set to 35 seconds (backend long-polls for 30 seconds max).
3. **Handling Server Responses**:
   - If response `status == "matched"`: Transition state to `Matched`, extract `match_id` and partner info, navigate to `ActiveChatScreen`.
   - If response `status == "retry"`: Immediately re-trigger `POST /v1/matching/start` if user has not canceled searching.
   - On network error or timeout: Exponential backoff delay (1s, 2s, 4s) before retry.

### Phase 5: WebSocket Connection Hub & Notification Engine
1. **Connection Handshake**: Establish WebSocket connection at `ws://<host>:<port>/v1/ws?token=<JWT>` or with Bearer header.
2. **Event Parsing**: Listen on Stream channel for inbound JSON payloads:
   - `friend_request_received`: Trigger a dynamic overlay toast / snackbar notification for incoming friend request.
   - `chat_message`: Append message to current chat feed.
   - `typing_indicator`: Display typing status in active chat header.
3. **Heartbeat / Ping**: Maintain WS connection health with automatic reconnect logic on disconnect.

### Phase 6: Friendship Subsystem & Social UI
1. **Send Friend Request**: Call `POST /v1/friendship/requests` with `{ "receiver_id": "<partner_user_id>" }`.
2. **Pending Requests List**: Fetch from `GET /v1/friendship/requests`. Display red badge count on Friends tab.
3. **Accept / Decline Actions**:
   - Call `POST /v1/friendship/requests/:requestId/accept` or `decline`.
   - Refresh local friendship state immediately on success.
4. **Friends List & Unfriend**: Fetch active list via `GET /v1/friendship/friends`. Enable sliding swipe-to-delete to call `DELETE /v1/friendship/friends/:id`.

---

## 🎨 Design System & UI Aesthetics Checklist

- **Color Palette**:
  - Dark Primary Background: `#0F172A` (Slate 900)
  - Card/Surface Glass: `#1E293B` (Slate 800 with 80% opacity and `BackdropFilter` blur)
  - Accent Pulse Neon: `#6366F1` (Indigo Neon) & `#EC4899` (Pink Vibrant)
  - Online Status Green: `#10B981` (Emerald 500)
- **Matchmaking Radar Screen**:
  - Animated pulsing concentric circles centered around user avatar during matchmaking.
  - Floating pills showing selected interests and gender target.
  - Cancel button with immediate request cancellation context.
- **Active Chat Screen**:
  - Split view: Top window for WebRTC video preview / placeholder avatar, bottom window for live scrollable text chat.
  - Quick action bar: "Add Friend", "Report", "Skip / Next Match".

---

## 🔍 Verification & Testing Strategy for the Agent

When building or testing the Flutter app:
1. Validate token storage by inspecting secure storage after onboarding.
2. Test Matchmaking long-polling with two concurrent Flutter emulator instances or backend curl scripts.
3. Verify presence heartbeat triggers in network logs every 15 seconds.
4. Ensure WebSocket reconnection fires smoothly when switching airplane mode on/off.

---

> [!IMPORTANT]
> Always consult the **[RanChat Backend & Flutter Reference File](file:///c:/Users/mayank/Desktop/goprojects/ranchat/.agents/skills/flutter-ranchat-frontend/references/ranchat_backend_flutter_ref.md)** for exact JSON contracts, schemas, and sequence flows before writing Dart code.
