# 🧭 RanChat Flutter Front-End Agent — Master Index & Technical Entry Point

> **🤖 FOR AI AGENTS:** This single document is your **Master Entry Point** to build the Flutter mobile & web client for **RanChat**. By reading this master file, you are automatically directed to inspect the three core system specification files below, which contain all necessary architectural details, schemas, domain rules, and backend audits.

---

## 📌 1. Primary Reference Files Index

Before generating code or designing Flutter features, you **MUST** read and cross-reference these 3 primary documentation files:

| # | Reference File | Focus Area & Description | Key Sections to Inspect |
|---|----------------|--------------------------|-------------------------|
| **1** | 📄 **[PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md)** | **Overall System Architecture & API Reference**<br>Covers DDD Bounded Contexts, Auth JWT flow, Redis Presence engine (`presence:<userID>`), MongoDB models (`users`, `friends`, `friendrequests`), domain event bus, and Gin routes. | • Section 2: Architectural Patterns<br>• Section 3: Database Schemas<br>• Section 6: Complete API Reference |
| **2** | 📄 **[matchmakingoverview.md](matchmakingoverview.md)** | **Matchmaking Engine Audit & Technical Spec**<br>Covers reciprocal preference matching (Interest, Gender, GenderPref), in-memory queues (`MatchmakingMemoryDB`), channel dispatch (`EnqueueChan`), 30s HTTP long-polling behavior, and concurrency safeguards. | • Section 2: Workflow Architecture<br>• Section 3: Technical Deep-Dive<br>• Section 5: Client Safeguards |
| **3** | 📄 **[friendshipoverview.md](friendshipoverview.md)** | **Friendship Subsystem & Real-Time Popups**<br>Covers friend request lifecycle (send, accept, decline, remove), active friends list, IDOR security considerations, and real-time WebSocket notifications (`WSHub`). | • Section 2: Workflow Architecture<br>• Section 3: Technical Audit<br>• Section 6: Endpoint Matrix |

> 💡 *Note: For pre-written Dart models and Dio HTTP interceptor code snippets, also see the Flutter Reference Manual in 📄 **[docs/ranchat_backend_flutter_ref.md](docs/ranchat_backend_flutter_ref.md)** and Antigravity Skill in 📄 **[.agents/skills/flutter-ranchat-frontend/SKILL.md](.agents/skills/flutter-ranchat-frontend/SKILL.md)**.*

---

## 🗺️ 2. Architectural Blueprint & Subsystem Cross-Reference

```
                                  ┌───────────────────────────┐
                                  │  FLUTTER AGENT MASTER     │
                                  └─────────────┬─────────────┘
                                                │
         ┌──────────────────────────────────────┼──────────────────────────────────────┐
         ▼                                      ▼                                      ▼
┌──────────────────────────┐          ┌──────────────────────────┐          ┌──────────────────────────┐
│   PROJECT_OVERVIEW.md    │          │  matchmakingoverview.md  │          │   friendshipoverview.md  │
├──────────────────────────┤          ├──────────────────────────┤          ├──────────────────────────┤
│ • Auth & User Profile    │          │ • Reciprocal Criteria    │          │ • Request Send/Accept    │
│ • Redis 15s Heartbeats   │          │ • 30s Long-Polling Loop  │          │ • Friends CRUD           │
│ • Base MongoDB Models    │          │ • Dio Timeout (35s)      │          │ • WebSocket Popups       │
└──────────────────────────┘          └──────────────────────────┘          └──────────────────────────┘
```

---

## 🚀 3. Step-by-Step Agent Implementation Roadmap

When you (the Agent) build the Flutter application, follow this exact sequence:

### Step 1: Authentication & User Registration Context
- **Refer to:** Section 6 of [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md)
- **Tasks:**
  1. Generate unique `deviceId` via `flutter_secure_storage` or `uuid`.
  2. Implement `POST /v1/auth/signup` with payload `{ name, age, gender, bio, interest, deviceId }`.
  3. Store JWT token securely and attach to Dio `Authorization: Bearer <token>` header.

### Step 2: Presence Engine & Background Heartbeat
- **Refer to:** Section 3 & 6 of [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md)
- **Tasks:**
  1. Trigger `POST /v1/presence` upon login/signup to initialize presence in Redis (`presence:<userID>`).
  2. Start a background `Timer.periodic(Duration(seconds: 15))` calling `POST /v1/presence/heartbeat` (Redis TTL is 20s).
  3. Call `PATCH /v1/presence/update` when user starts typing or enters an active chat.

### Step 3: Matchmaking Long-Polling UI & State Machine
- **Refer to:** Section 2 & 3 of [matchmakingoverview.md](matchmakingoverview.md)
- **Tasks:**
  1. Configure Dio `receiveTimeout` to **35 seconds** (backend long-polls up to 30s).
  2. Implement `POST /v1/matching/start` with payload `{ interest, gender, gender_pref }`.
  3. Handle response `status == "matched"` by navigating to `ActiveChatScreen`.
  4. Handle response `status == "retry"` by auto-re-querying if the user is still on the Radar screen.

### Step 4: Friendship & WebSocket Notification Center
- **Refer to:** Section 2 & 3 of [friendshipoverview.md](friendshipoverview.md)
- **Tasks:**
  1. Connect to WebSocket at `ws://<host>:<port>/v1/ws?token=<JWT>`.
  2. Listen for inbound `friend_request_received` events and trigger in-app notification toasts.
  3. Implement Friend Requests UI: `POST /v1/friendship/requests`, `GET /v1/friendship/requests`, `POST /accept`, `POST /decline`.
  4. Implement Active Friends List: `GET /v1/friendship/friends`, `DELETE /v1/friendship/friends/:id`.

---

## 🔒 4. Summary of Critical Domain Rules for the Agent

1. **Long-Polling Timeout Safety:** Dio HTTP `receiveTimeout` **must be set to at least 35 seconds**. Setting it below 30s will break matchmaking.
2. **Heartbeat Frequency:** Presence heartbeat MUST run every **15 seconds** to prevent Redis key expiration (20s TTL).
3. **IDOR Defense on Client:** Always check `request.receiver_id == currentUserId` before displaying accept/decline buttons in the UI.
4. **WebSocket Auto-Reconnect:** On network loss, automatically attempt to reconnect to `/v1/ws` with exponential backoff.

---

> 📖 **Summary:** Tell your agent to open **`FLUTTER_AGENT_MASTER.md`**. It links directly to [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md), [matchmakingoverview.md](matchmakingoverview.md), and [friendshipoverview.md](friendshipoverview.md).
