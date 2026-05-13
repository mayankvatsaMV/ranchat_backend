// friend_service.go — Business logic for the friend request system.
//
// This service bridges:
//   - repository.friendship_repository  (MongoDB persistence)
//   - repository.ws_room_repository     (RAM WebSocket rooms)
//   - repository.match_repository       (validate active match state)
package service

import (
	"context"
	"errors"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"ranchat_backend/models"
	"ranchat_backend/repository"
)

var ErrNotInSameMatch = errors.New("you and the recipient are not in the same active match")

// SendFriendRequest validates match state, persists the request, and pushes a
// realtime WebSocket event to the partner if they are connected.
func SendFriendRequest(ctx context.Context, senderID, matchID string) (*models.FriendRequest, int, error) {
	senderOID, err := primitive.ObjectIDFromHex(senderID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid senderId")
	}
	// 1. Validate: sender must be part of this active match
	match := repository.GetMatchByUser(senderID)
	if match == nil || match.MatchID != matchID {
		return nil, http.StatusForbidden, ErrNotInSameMatch
	}

	// 2. Determine the partner's ID
	partnerID := match.User1ID
	if match.User1ID == senderID {
		partnerID = match.User2ID
	}
	partnerOID, _ := primitive.ObjectIDFromHex(partnerID)

	// 3. Persist in MongoDB
	req, err := repository.CreateFriendRequest(ctx, senderOID, partnerOID, matchID)
	if err != nil {
		switch err {
		case repository.ErrDuplicateFriendRequest:
			return nil, http.StatusConflict, err
		case repository.ErrAlreadyFriends:
			return nil, http.StatusConflict, err
		default:
			return nil, http.StatusInternalServerError, err
		}
	}

	// 4. Push realtime event to partner (best-effort — partner may not be WS-connected yet)
	partnerClient := repository.GetPartner(matchID, senderID)
	if partnerClient != nil {
		repository.SendToClient(partnerClient, models.WSMessage{
			Type: models.WSEventFriendRequestReceived,
			Data: models.WSFriendRequestPayload{
				RequestID: req.ID.Hex(),
				FromID:    senderID,
				MatchID:   matchID,
			},
		})
	}

	return req, http.StatusCreated, nil
}

// ResolveFriendRequest accepts or rejects a pending request, persists the
// state change, and pushes a realtime event back to the original sender.
func ResolveFriendRequest(ctx context.Context, resolverID, requestID string, accept bool) (*models.FriendRequest, *models.Friendship, int, error) {
	resolverOID, err := primitive.ObjectIDFromHex(resolverID)
	if err != nil {
		return nil, nil, http.StatusBadRequest, errors.New("invalid userId")
	}
	reqOID, err := primitive.ObjectIDFromHex(requestID)
	if err != nil {
		return nil, nil, http.StatusBadRequest, errors.New("invalid requestId")
	}

	req, friendship, err := repository.ResolveFriendRequest(ctx, reqOID, resolverOID, accept)
	if err != nil {
		switch err {
		case repository.ErrFriendRequestNotFound:
			return nil, nil, http.StatusNotFound, err
		case repository.ErrNotAuthorized:
			return nil, nil, http.StatusForbidden, err
		case repository.ErrRequestNotPending:
			return nil, nil, http.StatusConflict, err
		default:
			return nil, nil, http.StatusInternalServerError, err
		}
	}

	// Push realtime notification to the original sender
	senderID := req.FromID.Hex()
	senderClient := repository.GetPartner(req.MatchID, resolverID)
	if senderClient != nil && senderClient.UserID == senderID {
		eventType := models.WSEventFriendRequestRejected
		var data interface{}
		if accept && friendship != nil {
			eventType = models.WSEventFriendRequestAccepted
			data = models.WSAcceptPayload{
				RequestID:    req.ID.Hex(),
				FriendshipID: friendship.ID.Hex(),
			}
		}
		repository.SendToClient(senderClient, models.WSMessage{
			Type: eventType,
			Data: data,
		})
	}

	return req, friendship, http.StatusOK, nil
}

// GetMyFriendRequests returns all pending requests the caller has received.
func GetMyFriendRequests(ctx context.Context, userID string) ([]models.FriendRequest, int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId")
	}
	requests, err := repository.GetMyFriendRequests(ctx, oid)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return requests, http.StatusOK, nil
}

// GetMyFriends returns all accepted friendships for the caller.
func GetMyFriends(ctx context.Context, userID string) ([]models.Friendship, int, error) {
	oid, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return nil, http.StatusBadRequest, errors.New("invalid userId")
	}
	friends, err := repository.GetMyFriends(ctx, oid)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return friends, http.StatusOK, nil
}
