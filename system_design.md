# Ranchat Backend — System Design Document

This document outlines the High-Level Design (HLD) and Low-Level Design (LLD) of the Ranchat backend. The system is designed to handle highly concurrent, real-time matchmaking and presence tracking with minimal database overhead.

---

## 1. System Overview

Ranchat is a real-time anonymous matchmaking backend. The primary design goal is **high performance and low latency**. To achieve this, the architecture deliberately splits data between persistent storage (MongoDB) and volatile in-memory storage (Go RAM), minimizing database I/O for real-time states like "searching", "typing", and "online".

### 1.1. Technology Stack
*   **Language:** Go (Golang) — chosen for its lightweight goroutines and built-in concurrency primitives (Channels, Mutexes).
*   **Framework:** Gin — a high-performance HTTP web framework for routing and middleware.
*   **Database:** MongoDB — NoSQL database used strictly for persistent user profiles.
*   **Real-time Protocol:** 
    *   **Long-Polling:** Used exclusively for the matchmaking queue to keep HTTP connections open and wait for partners without constantly hammering the server with polling requests.

---

## 2. High-Level Design (HLD)

The system follows a standard Layered (N-Tier) Monolithic Architecture, divided into distinct concerns to ensure decoupling.

### 2.1. Component Layers
1.  **Transport / Controller Layer (`/controller`)**: Parses incoming HTTP requests, validates inputs, and extracts JWT data.
2.  **Service / Business Logic Layer (`/service`)**: Orchestrates rules (e.g., "if matched, set match state and update presence").
3.  **Repository / Data Layer (`/repository`)**: Abstracted data access. Controllers and Services don't know *how* data is stored.
4.  **Storage Engine**:
    *   `user_repository.go` talks to **MongoDB**.
    *   `match_repository.go` & `presence_repository.go` talk to **RAM**.

### 2.2. Dual Storage Strategy
A critical architectural decision was to split the storage layer:
*   **Persistent (MongoDB):** Stores `User` documents. Read/writes happen on login or profile updates (low frequency).
*   **Volatile (Go RAM):** Stores `Presence` and `Match` states. Read/writes happen thousands of times a minute (e.g., heartbeats, queue entries). If the server restarts, this data is meant to be wiped anyway.

---

## 3. Low-Level Design (LLD)

### 3.1. Authentication System
*   **Anonymous Login Flow:** The client sends a unique `deviceId`. If the `deviceId` exists in MongoDB, it logs the user in. If not, it creates a new account.
*   **Stateless Security:** The backend returns a JWT (JSON Web Token) containing the `userID` and `deviceId`. The backend does not store sessions; the `middleware/auth.go` intercepts requests, validates the cryptographic signature of the JWT, and injects the `userID` into the Gin context.

### 3.2. Presence Engine
Presence represents whether a user is online, searching, or typing.
*   **Data Structure:** A global `map[string]*models.Presence` protected by a `sync.RWMutex`.
*   **Heartbeat Mechanism:** The client calls `POST /presence/heartbeat` every ~20 seconds. The repository applies an immediate write-lock (`mu.Lock()`) and updates `lastActiveAt`.
*   **Concurrency Control:** Because presence is read heavily (by matchmaking, by profile views) but written frequently, the `sync.RWMutex` allows multiple concurrent readers but exclusive writers.

### 3.3. Matchmaking Engine (The Core)
The matchmaking system is fully in-memory and combines **Queue Arrays**, **Maps**, and **Go Channels** to create a highly concurrent, thread-safe long-polling system.

#### Core Data Structures (`match_repository.go`):
1.  **Queue (`[]string`)**: A simple FIFO array holding `userIDs` of people waiting. Protected by `queue.mu`.
2.  **Waiters Registry (`map[string]chan MatchResult`)**: When a user enters the queue, they register a Go channel. Their HTTP request blocks by listening to this channel. Protected by `waiters.mu`.
3.  **Active Matches (`map[string]*models.Match`)**: Holds active chat sessions. Protected by `activeMatches.mu`.

#### The Long-Polling Flow:
1.  **User A Calls `/matches/search`:**
    *   Marks `isSearching = true` in the Presence store.
    *   Registers a waiter channel.
    *   Acquires queue lock. Queue is empty. Adds User A to the array. Releases lock.
    *   HTTP handler blocks, waiting for data on User A's channel.
2.  **User B Calls `/matches/search`:**
    *   Marks `isSearching = true` in the Presence store.
    *   Registers a waiter channel.
    *   Acquires queue lock. Queue has User A.
    *   Pops User A from the array. Releases lock.
    *   Creates a `Match` object.
    *   **Signals User A's Channel:** Pushes the Match result into User A's waiting channel.
3.  **Resolution:**
    *   User A's blocked HTTP request receives the channel signal and instantly returns `200 OK` to the client.
    *   User B's HTTP request receives an instant match and returns `200 OK`.
4.  **Timeouts & Disconnects:** 
    *   If User A sits in the queue for 30 seconds without a match, a `select` statement triggers a timeout, removes User A from the queue, and returns `202 Accepted`.
    *   If User A closes the app or drops connection, the HTTP context dies, and the `CancelSearch` cleanup function runs automatically.

---

## 4. Concurrency & Thread-Safety Rules
To prevent race conditions and deadlocks, the system enforces strict locking hierarchies:
1.  Locks are scoped exclusively to their respective structs (`presenceStore.mu`, `matchQueue.mu`, `waiters.mu`).
2.  **No Nested Locking:** A lock on `matchQueue` is always released *before* calling a function that might acquire a lock on `waiters.mu` or `activeMatches.mu`. This completely eliminates the possibility of deadlocks during high-throughput matchmaking.
