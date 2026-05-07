// match_repository.go — in-memory match queue and active match store.
//
// Three data structures:
//  1. matchQueue    — FIFO list of users waiting for a partner.
//  2. matchStore    — active matches (matchID → Match, userID → matchID).
//  3. waiterRegistry — channels that long-poll HTTP handlers block on.
//
// Lock order rule: queue.mu → never nests with matchStore.mu or waiters.mu.
// This prevents deadlocks.
package repository

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"ranchat_backend/models"
)

var ErrNotInMatch = errors.New("user is not in an active match")

// ─── Match ID generator ───────────────────────────────────────────────────────

func newMatchID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ─── Waiter Registry ──────────────────────────────────────────────────────────
// When a user is queued and waiting, they register a channel here.
// When their partner arrives and matches them, the channel is signalled.
// The HTTP handler blocks on this channel — this is the long-polling mechanism.

// MatchResult is sent through the waiter channel when a match is made.
type MatchResult struct {
	MatchID   string
	PartnerID string
}

type waiterRegistry struct {
	mu sync.Mutex
	m  map[string]chan MatchResult // userID → channel
}

var waiters = &waiterRegistry{m: make(map[string]chan MatchResult)}

// RegisterWaiter creates a buffered channel for a queued user.
// Call this BEFORE Enqueue so the channel is ready before a partner can arrive.
func RegisterWaiter(userID string) chan MatchResult {
	ch := make(chan MatchResult, 1) // buffered so sender never blocks
	waiters.mu.Lock()
	waiters.m[userID] = ch
	waiters.mu.Unlock()
	return ch
}

// RemoveWaiter cleans up a user's channel (called on timeout or cancel).
func RemoveWaiter(userID string) {
	waiters.mu.Lock()
	delete(waiters.m, userID)
	waiters.mu.Unlock()
}

// notifyWaiter signals a waiting user's channel with their match result.
// Called by storeMatch when the waiting user finally gets a partner.
func notifyWaiter(userID string, result MatchResult) {
	waiters.mu.Lock()
	ch, ok := waiters.m[userID]
	if ok {
		delete(waiters.m, userID)
	}
	waiters.mu.Unlock()

	if ok {
		select {
		case ch <- result: // non-blocking send (buffer size 1)
		default:
		}
	}
}

// ─── Match Queue ──────────────────────────────────────────────────────────────

type matchQueue struct {
	mu    sync.Mutex
	users []string // FIFO — first in, first out
}

var queue = &matchQueue{}

// Enqueue tries to match the user immediately with whoever is waiting.
// Returns (matchID, partnerID, true) if matched instantly,
// or ("", "", false) if the user is now in the queue waiting.
// Queue lock is released BEFORE touching matchStore to prevent deadlocks.
func Enqueue(userID string) (string, string, bool) {
	queue.mu.Lock()

	// Guard: skip if already in queue
	for _, id := range queue.users {
		if id == userID {
			queue.mu.Unlock()
			return "", "", false
		}
	}

	if len(queue.users) == 0 {
		// No one waiting — add user to queue
		queue.users = append(queue.users, userID)
		queue.mu.Unlock()
		return "", "", false
	}

	// Someone is waiting — pop them and form a match
	partnerID := queue.users[0]
	queue.users = queue.users[1:]
	queue.mu.Unlock() // release queue lock BEFORE storeMatch

	matchID := storeMatch(userID, partnerID)
	return matchID, partnerID, true
}

// Dequeue removes a user from the waiting queue (search cancelled or disconnect).
func Dequeue(userID string) {
	queue.mu.Lock()
	defer queue.mu.Unlock()

	for i, id := range queue.users {
		if id == userID {
			queue.users = append(queue.users[:i], queue.users[i+1:]...)
			return
		}
	}
}

// ─── Active Match Store ───────────────────────────────────────────────────────

type matchStore struct {
	mu      sync.RWMutex
	matches map[string]*models.Match // matchID → Match
	byUser  map[string]string        // userID  → matchID
}

var activeMatches = &matchStore{
	matches: make(map[string]*models.Match),
	byUser:  make(map[string]string),
}

// storeMatch creates a Match, saves it, and notifies the waiting user (user1ID)
// via their waiter channel so their long-polling HTTP request can resolve.
func storeMatch(user1ID, user2ID string) string {
	m := &models.Match{
		MatchID:   newMatchID(),
		User1ID:   user1ID,
		User2ID:   user2ID,
		StartedAt: time.Now().UTC(),
	}

	activeMatches.mu.Lock()
	activeMatches.matches[m.MatchID] = m
	activeMatches.byUser[user1ID] = m.MatchID
	activeMatches.byUser[user2ID] = m.MatchID
	activeMatches.mu.Unlock()

	// Notify the user who was sitting in queue waiting —
	// this unblocks their long-polling POST /matches/search request.
	// user2ID is the one popped from the queue, user1ID is the new arrival.
	notifyWaiter(user2ID, MatchResult{MatchID: m.MatchID, PartnerID: user1ID})

	return m.MatchID
}

// GetMatchByUser returns the active Match for a user, or nil if not in one.
func GetMatchByUser(userID string) *models.Match {
	activeMatches.mu.RLock()
	defer activeMatches.mu.RUnlock()

	mid, ok := activeMatches.byUser[userID]
	if !ok {
		return nil
	}
	m := activeMatches.matches[mid]
	if m == nil {
		return nil
	}
	cp := *m
	return &cp
}

// EndMatch removes the active match for a user and returns the partner's ID.
func EndMatch(userID string) (partnerID string, err error) {
	activeMatches.mu.Lock()
	defer activeMatches.mu.Unlock()

	mid, ok := activeMatches.byUser[userID]
	if !ok {
		return "", ErrNotInMatch
	}

	m := activeMatches.matches[mid]
	if m == nil {
		return "", ErrNotInMatch
	}

	if m.User1ID == userID {
		partnerID = m.User2ID
	} else {
		partnerID = m.User1ID
	}

	delete(activeMatches.matches, mid)
	delete(activeMatches.byUser, m.User1ID)
	delete(activeMatches.byUser, m.User2ID)

	return partnerID, nil
}
