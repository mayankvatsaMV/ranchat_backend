// presence_service.go — business logic for presence operations.
package service

import (
	"context"
	"errors"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"ranchat_backend/models"
	"ranchat_backend/repository"
)

// Heartbeat updates the user's lastActiveAt timestamp.
// The client should call this every ~20 seconds while the app is in the foreground.
func Heartbeat(ctx context.Context, userID string) (int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return http.StatusBadRequest, errors.New("invalid userId")
	}

	if err := repository.Heartbeat(ctx, oid); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

// GetPresence returns the current presence state for any user by their ID.
func GetPresence(ctx context.Context, userID string) (*models.Presence, int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId format")
	}

	p, err := repository.FindPresenceByUserID(ctx, oid)
	if err != nil {
		if err == repository.ErrPresenceNotFound {
			return nil, http.StatusNotFound, errors.New("presence not found — has this user logged in?")
		}
		return nil, http.StatusInternalServerError, err
	}

	return p, http.StatusOK, nil
}

// UpdatePresence applies only the fields the client actually sent (PATCH semantics).
// When isSearching transitions to true  → enters the match queue.
// When isSearching transitions to false → leaves the match queue.
func UpdatePresence(ctx context.Context, userID string, req *models.UpdatePresenceRequest) (*models.Presence, int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId")
	}

	fields := repository.UpdatePresenceFields{}
	updatedCount := 0

	if req.IsSearching != nil {
		fields.IsSearching = req.IsSearching
		updatedCount++
	}
	if req.ActiveChatID != nil {
		fields.ActiveChatID = req.ActiveChatID
		updatedCount++
	}
	if req.TypingTo != nil {
		fields.TypingTo = req.TypingTo
		updatedCount++
	}

	if updatedCount == 0 {
		return nil, http.StatusBadRequest, errors.New("no fields provided to update")
	}

	updated, err := repository.UpdatePresence(ctx, oid, fields)
	if err != nil {
		if err == repository.ErrPresenceNotFound {
			return nil, http.StatusNotFound, errors.New("presence not found — has this user logged in?")
		}
		return nil, http.StatusInternalServerError, err
	}

	// Trigger matchmaking when isSearching changes
	if req.IsSearching != nil {
		if *req.IsSearching {
			go StartSearch(ctx, userID) // non-blocking — response returns immediately
		} else {
			go CancelSearch(ctx, userID)
		}
	}

	return updated, http.StatusOK, nil
}
