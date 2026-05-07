// match_service.go — matchmaking business logic.
package service

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	
	"ranchat_backend/models"
	"ranchat_backend/repository"
)

// notifyBothAndUpdatePresence is a shared helper that runs after a match is made.
// It updates presence in RAM.
func notifyBothAndUpdatePresence(ctx context.Context, userID, partnerID, matchID string) {
	userOID, _ := primitive.ObjectIDFromHex(userID)
	partnerOID, _ := primitive.ObjectIDFromHex(partnerID)

	_ = repository.SetMatched(ctx, userOID, partnerOID)
	_ = repository.SetMatched(ctx, partnerOID, userOID)
}

// EnqueueAndMatch is called by the HTTP handler (long-polling).
//
// Two possible outcomes:
//  1. Instant match  → returns (matchID, partnerID, nil channel) — handler responds 200 immediately.
//  2. No partner yet → returns ("", "", waitCh) — handler blocks on waitCh until a partner arrives
//     or the request times out. When User B arrives, storeMatch signals waitCh automatically.
func EnqueueAndMatch(ctx context.Context, userID string) (string, string, chan repository.MatchResult) {
	// Mark presence as searching so it reflects in GET /presence
	userOID, _ := primitive.ObjectIDFromHex(userID)
	isSearching := true
	_, _ = repository.UpdatePresence(ctx, userOID, repository.UpdatePresenceFields{IsSearching: &isSearching})

	// Register the waiter channel BEFORE enqueue so it is ready
	// the moment a partner could arrive (prevents a race).
	waitCh := repository.RegisterWaiter(userID)

	matchID, partnerID, matched := repository.Enqueue(userID)

	if matched {
		// Got a partner instantly — clean up the waiter channel we registered
		repository.RemoveWaiter(userID)
		notifyBothAndUpdatePresence(ctx, userID, partnerID, matchID)
		return matchID, partnerID, nil
	}

	// No partner yet — return the channel so the controller can block on it.
	// storeMatch() will signal this channel when a partner arrives.
	return "", "", waitCh
}

// StartSearch is the fire-and-forget version used by the presence hook (go routine).
// Since it runs in a goroutine, we don't use long-polling here.
func StartSearch(ctx context.Context, userID string) {
	matchID, partnerID, matched := repository.Enqueue(userID)
	if !matched {
		return
	}
	notifyBothAndUpdatePresence(ctx, userID, partnerID, matchID)
}

// CancelSearch removes the user from the waiting queue without matching.
// Called when user sets isSearching=false or disconnects while searching.
func CancelSearch(ctx context.Context, userID string) {
	repository.Dequeue(userID)
	repository.RemoveWaiter(userID)

	oid, _ := primitive.ObjectIDFromHex(userID)
	_ = repository.ClearMatchState(ctx, oid)
}

// EndMatch ends an active match and notifies the partner.
func EndMatch(ctx context.Context, userID string) error {
	partnerID, err := repository.EndMatch(userID)
	if err != nil {
		return err
	}

	userOID, _ := primitive.ObjectIDFromHex(userID)
	partnerOID, _ := primitive.ObjectIDFromHex(partnerID)

	_ = repository.ClearMatchState(ctx, userOID)
	_ = repository.ClearMatchState(ctx, partnerOID)

	// Clean up the WebSocket match room — all connected clients will receive
	// a close on their send channel and disconnect gracefully.
	repository.DeleteRoom(userID)

	return nil
}

// GetMyMatch returns the current active match for a user, or nil.
func GetMyMatch(userID string) *models.Match {
	return repository.GetMatchByUser(userID)
}
