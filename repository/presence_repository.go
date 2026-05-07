// presence_repository.go — IN-MEMORY storage for presence data.
//
// Why In-Memory?
// Realtime state (online/offline, typing, searching) changes thousands of
// times per second. Writing this to MongoDB is slow, costly, and unnecessary
// because if the server restarts, realtime state is meant to be reset anyway.
//
// By storing this in a RAM map, lookups and updates take nanoseconds instead
// of milliseconds, saving massive database load.
package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"ranchat_backend/models"
)

var ErrPresenceNotFound = errors.New("presence not found in memory")

// presenceStore holds all active user sessions in RAM.
// The RWMutex ensures safe concurrent access across thousands of goroutines.
type presenceStore struct {
	mu       sync.RWMutex
	sessions map[string]*models.Presence
}

// Global in-memory store
var store = &presenceStore{
	sessions: make(map[string]*models.Presence),
}

// UpsertPresence creates or updates a user's presence in RAM.
// Called every time a user logs in.
func UpsertPresence(ctx context.Context, userID primitive.ObjectID, deviceID string) (*models.Presence, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	uid := userID.Hex()
	now := time.Now().UTC()

	p, exists := store.sessions[uid]
	if !exists {
		// New session in memory
		p = &models.Presence{
			UserID:      userID,
			DeviceID:    deviceID,
			IsSearching: false,
			IsMatched:   false,
		}
		store.sessions[uid] = p
	}

	// Update active state
	p.DeviceID = deviceID
	p.LastActiveAt = now
	p.UpdatedAt = now

	// Return a copy so the caller doesn't accidentally mutate the memory store directly
	copy := *p
	return &copy, nil
}

// Heartbeat updates the lastActiveAt timestamp in RAM.
// Extremely fast operation — takes nanoseconds.
func Heartbeat(ctx context.Context, userID primitive.ObjectID) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	p, exists := store.sessions[userID.Hex()]
	if !exists {
		return ErrPresenceNotFound
	}

	now := time.Now().UTC()
	p.LastActiveAt = now
	p.UpdatedAt = now
	return nil
}

// FindPresenceByUserID reads the presence from RAM.
func FindPresenceByUserID(ctx context.Context, userID primitive.ObjectID) (*models.Presence, error) {
	store.mu.RLock() // RLock allows multiple readers at the same time
	defer store.mu.RUnlock()

	p, exists := store.sessions[userID.Hex()]
	if !exists {
		return nil, ErrPresenceNotFound
	}

	copy := *p
	return &copy, nil
}

// UpdatePresenceFields defines which fields can be updated in RAM.
type UpdatePresenceFields struct {
	IsSearching  *bool
	ActiveChatID *string
	TypingTo     *string
}

// UpdatePresence modifies specific fields for a user in RAM.
func UpdatePresence(ctx context.Context, userID primitive.ObjectID, fields UpdatePresenceFields) (*models.Presence, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	p, exists := store.sessions[userID.Hex()]
	if !exists {
		return nil, ErrPresenceNotFound
	}

	if fields.IsSearching != nil {
		p.IsSearching = *fields.IsSearching
	}
	if fields.ActiveChatID != nil {
		p.ActiveChatID = fields.ActiveChatID
	}
	if fields.TypingTo != nil {
		p.TypingTo = fields.TypingTo
	}

	p.UpdatedAt = time.Now().UTC()

	copy := *p
	return &copy, nil
}

// SetMatched marks a user as matched — called by the matchmaking system.
// Sets IsSearching=false, IsMatched=true, CurrentMatchID=partnerID.
func SetMatched(ctx context.Context, userID primitive.ObjectID, partnerID primitive.ObjectID) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	p, exists := store.sessions[userID.Hex()]
	if !exists {
		return ErrPresenceNotFound
	}

	p.IsSearching = false
	p.IsMatched = true
	p.CurrentMatchID = &partnerID
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// ClearMatchState resets a user back to idle — called when a match ends or search is cancelled.
func ClearMatchState(ctx context.Context, userID primitive.ObjectID) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	p, exists := store.sessions[userID.Hex()]
	if !exists {
		return ErrPresenceNotFound
	}

	p.IsSearching = false
	p.IsMatched = false
	p.CurrentMatchID = nil
	p.ActiveChatID = nil
	p.TypingTo = nil
	p.UpdatedAt = time.Now().UTC()
	return nil
}
