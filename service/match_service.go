// match_service.go — matchmaking business logic.
package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"ranchat_backend/models"
	"ranchat_backend/repository"
)

// notifyBothAndUpdatePresence is a shared helper that runs after a match is made.
// It marks both users as matched in the RAM presence store.
func notifyBothAndUpdatePresence(ctx context.Context, userID, partnerID, matchID string) {
	userOID, _ := primitive.ObjectIDFromHex(userID)
	partnerOID, _ := primitive.ObjectIDFromHex(partnerID)

	_ = repository.SetMatched(ctx, userOID, partnerOID)
	_ = repository.SetMatched(ctx, partnerOID, userOID)
}

// EnqueueAndMatch is called by the HTTP handler (long-polling).
//
// prefs contains the caller's search filters (e.g. wantGender).
// The user's own gender is snapshotted from their profile before entering the queue
// so the matching hot path never needs a database call.
//
// Two possible outcomes:
//  1. Instant match  → returns (matchID, partnerID, nil channel) — handler responds 200 immediately.
//  2. No partner yet → returns ("", "", waitCh) — handler blocks on waitCh until a partner arrives
//     or the request times out. When User B arrives, storeMatch signals waitCh automatically.
func EnqueueAndMatch(ctx context.Context, userID string, prefs models.SearchPreferences) (string, string, chan repository.MatchResult) {
	// Mark presence as searching so it reflects in GET /presence.
	userOID, _ := primitive.ObjectIDFromHex(userID)
	isSearching := true
	_, _ = repository.UpdatePresence(ctx, userOID, repository.UpdatePresenceFields{IsSearching: &isSearching})

	// Fetch the user's profile to snapshot their gender for the queue entry.
	// This is a RAM read (presence) + one MongoDB read — acceptable because it
	// happens once per search start, not on every match attempt.
	user, err := repository.FindUserByID(ctx, userOID)
	if err != nil {
		// If we can't resolve the user, clean up and bail out.
		isSearching = false
		_, _ = repository.UpdatePresence(ctx, userOID, repository.UpdatePresenceFields{IsSearching: &isSearching})
		return "", "", nil
	}

	// Register the waiter channel BEFORE enqueue so it is ready
	// the moment a partner could arrive (prevents a race).
	waitCh := repository.RegisterWaiter(userID)

	incoming := &repository.WaitingUser{
		UserID:     userID,
		Gender:     user.Gender,
		Prefs:      prefs,
		EnqueuedAt: time.Now().UTC(),
	}

	matchID, partnerID, matched := repository.Enqueue(incoming)

	if matched {
		// Got a partner instantly — clean up the waiter channel we registered.
		repository.RemoveWaiter(userID)
		notifyBothAndUpdatePresence(ctx, userID, partnerID, matchID)
		return matchID, partnerID, nil
	}

	// No partner yet — return the channel so the controller can block on it.
	// storeMatch() will signal this channel when a partner arrives.
	return "", "", waitCh
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

// StartSearch is the fire-and-forget version triggered by the presence hook
// (PATCH /presence with isSearching=true). Because the presence endpoint has no
// search-preferences body, it defaults to PrefAnyone — the user can set gender
// filters only through the dedicated POST /matches/search endpoint.
func StartSearch(ctx context.Context, userID string) {
	prefs := models.SearchPreferences{WantGender: models.PrefAnyone}
	// EnqueueAndMatch already handles the presence flag and the waiter channel.
	// We discard the channel because this is fire-and-forget — no long-poll here.
	_, _, _ = EnqueueAndMatch(ctx, userID, prefs)
}
