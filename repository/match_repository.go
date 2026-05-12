// match_repository.go — in-memory match queue and active match store.
//
// Data structures:
//  1. genderPartitions — three sharded FIFO queues, one per gender bucket
//     (male / female / other). Each shard has its own mutex,
//     so male-only searches never block female-only searches.
//  2. queuedSet        — sync.Map for O(1) duplicate-enqueue detection.
//  3. activeMatches    — active matches (matchID → Match, userID → matchID).
//  4. waiterRegistry   — channels that long-poll HTTP handlers block on.
//
// Lock order rule (deadlock prevention):
//
//	Shards are always acquired in ascending index order: 0 (male) → 1 (female) → 2 (other).
//	activeMatches.mu is never held while any shard.mu is held.
//	waiters.mu is never held while activeMatches.mu is held.
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

// ─── Partitioned Match Queue ──────────────────────────────────────────────────
//
// Why partitioned?
// A single flat queue would require a global lock for every enqueue/dequeue,
// becoming a bottleneck at scale. Splitting by gender means a male-only search
// only locks the male shard — female-only searches proceed in parallel.

// WaitingUser is the queue entry. Gender is snapshotted from User.Gender at
// enqueue time so the matching hot path never touches MongoDB.
type WaitingUser struct {
	UserID     string
	Gender     string // snapshot of models.User.Gender ("male" | "female" | "other")
	Prefs      models.SearchPreferences
	EnqueuedAt time.Time
}

// queueShard is a single gender partition — its own lock and FIFO slice.
type queueShard struct {
	mu    sync.Mutex
	users []*WaitingUser
}

// Shard index constants — always lock in ascending order to prevent deadlocks.
const (
	shardMale   = 0
	shardFemale = 1
	shardOther  = 2
)

// genderPartitions holds the three gender-partitioned queues.
var genderPartitions = [3]*queueShard{
	{}, // index 0 — male waiters
	{}, // index 1 — female waiters
	{}, // index 2 — other waiters
}

// queuedSet provides O(1) duplicate-enqueue detection without scanning slices.
// Key: userID (string), Value: struct{}{}
var queuedSet sync.Map

// shardIndex maps a gender string to its shard index.
func shardIndex(gender string) int {
	switch gender {
	case "male":
		return shardMale
	case "female":
		return shardFemale
	default:
		return shardOther
	}
}

// shardsToSearch returns the shard indices to scan when looking for a partner,
// always in ascending order so lock acquisition is deadlock-safe.
func shardsToSearch(want models.GenderPreference) []int {
	switch want {
	case models.PrefMale:
		return []int{shardMale}
	case models.PrefFemale:
		return []int{shardFemale}
	case models.PrefOther:
		return []int{shardOther}
	default: // PrefAnyone or empty — search all shards
		return []int{shardMale, shardFemale, shardOther}
	}
}

// isCompatible returns true only when both users mutually accept each other's gender.
// This must hold in BOTH directions to prevent one-sided mismatches.
func isCompatible(candidate, incoming *WaitingUser) bool {
	// Does the incoming user accept the candidate's gender?
	if !genderAccepts(incoming.Prefs.WantGender, candidate.Gender) {
		return false
	}
	// Does the candidate accept the incoming user's gender?
	if !genderAccepts(candidate.Prefs.WantGender, incoming.Gender) {
		return false
	}
	return true
}

// genderAccepts returns true if pref allows targetGender.
func genderAccepts(pref models.GenderPreference, targetGender string) bool {
	if pref == models.PrefAnyone || pref == "" {
		return true
	}
	return string(pref) == targetGender
}

// Enqueue tries to find a compatible partner for the incoming user.
//
// Returns (matchID, partnerID, true) if matched instantly.
// Returns ("", "", false) if the user is now sitting in the queue.
//
// Lock order: shards are acquired in ascending index order, then released before
// touching activeMatches (inside storeMatch). This prevents all deadlock cycles.
func Enqueue(incoming *WaitingUser) (string, string, bool) {
	// O(1) duplicate guard — atomic LoadOrStore.
	if _, alreadyQueued := queuedSet.LoadOrStore(incoming.UserID, struct{}{}); alreadyQueued {
		return "", "", false
	}

	// Scan the shards that satisfy the incoming user's preference.
	for _, idx := range shardsToSearch(incoming.Prefs.WantGender) {
		shard := genderPartitions[idx]
		shard.mu.Lock()

		for i, candidate := range shard.users {
			if isCompatible(candidate, incoming) {
				// Remove candidate from the shard.
				shard.users = append(shard.users[:i], shard.users[i+1:]...)
				shard.mu.Unlock()
				// Clean up the candidate's queuedSet entry — they are no longer waiting.
				queuedSet.Delete(candidate.UserID)
				matchID := storeMatch(incoming.UserID, candidate.UserID)
				return matchID, candidate.UserID, true
			}
		}

		shard.mu.Unlock()
	}

	// No compatible partner found — place the incoming user in their gender shard.
	idx := shardIndex(incoming.Gender)
	shard := genderPartitions[idx]
	shard.mu.Lock()
	shard.users = append(shard.users, incoming)
	shard.mu.Unlock()

	return "", "", false
}

// Dequeue removes a user from whichever shard they are waiting in.
// Called when the user cancels search or disconnects.
func Dequeue(userID string) {
	// Fast path: if not in the set, skip shard scans entirely.
	if _, found := queuedSet.LoadAndDelete(userID); !found {
		return
	}

	// Scan all shards (ascending order, consistent with Enqueue).
	for _, shard := range genderPartitions {
		shard.mu.Lock()
		for i, u := range shard.users {
			if u.UserID == userID {
				shard.users = append(shard.users[:i], shard.users[i+1:]...)
				shard.mu.Unlock()
				return
			}
		}
		shard.mu.Unlock()
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

// storeMatch creates a Match, saves it, and notifies the waiting user (candidateID)
// via their waiter channel so their long-polling HTTP request can resolve.
func storeMatch(incomingID, candidateID string) string {
	m := &models.Match{
		MatchID:   newMatchID(),
		User1ID:   incomingID,
		User2ID:   candidateID,
		StartedAt: time.Now().UTC(),
	}

	activeMatches.mu.Lock()
	activeMatches.matches[m.MatchID] = m
	activeMatches.byUser[incomingID] = m.MatchID
	activeMatches.byUser[candidateID] = m.MatchID
	activeMatches.mu.Unlock()

	// Notify the user who was sitting in the queue (candidateID) —
	// this unblocks their long-polling POST /matches/search request.
	notifyWaiter(candidateID, MatchResult{MatchID: m.MatchID, PartnerID: incomingID})

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
