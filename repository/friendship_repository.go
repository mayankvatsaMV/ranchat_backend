// friendship_repository.go — MongoDB persistence for friend requests and friendships.
package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"ranchat_backend/config"
	"ranchat_backend/models"
)

var ErrFriendRequestNotFound  = errors.New("friend request not found")
var ErrAlreadyFriends         = errors.New("users are already friends")
var ErrDuplicateFriendRequest = errors.New("friend request already sent")
var ErrNotAuthorized          = errors.New("not authorized to perform this action")
var ErrRequestNotPending      = errors.New("friend request is not in pending state")

func friendRequestCol() *mongo.Collection {
	return config.GetCollection("friend_requests")
}

func friendshipCol() *mongo.Collection {
	return config.GetCollection("friendships")
}

// ── Friend Requests ───────────────────────────────────────────────────────────

// CreateFriendRequest inserts a new pending request.
// Returns ErrDuplicateFriendRequest if a pending request already exists for this match pair.
func CreateFriendRequest(ctx context.Context, fromID, toID primitive.ObjectID, matchID string) (*models.FriendRequest, error) {
	// Guard: no duplicate pending requests in the same match
	existing := &models.FriendRequest{}
	err := friendRequestCol().FindOne(ctx, bson.M{
		"matchId": matchID,
		"fromId":  fromID,
		"status":  models.FRStatusPending,
	}).Decode(existing)
	if err == nil {
		return nil, ErrDuplicateFriendRequest
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Guard: already friends
	if alreadyFriends, _ := AreFriends(ctx, fromID, toID); alreadyFriends {
		return nil, ErrAlreadyFriends
	}

	now := time.Now().UTC()
	req := &models.FriendRequest{
		ID:        primitive.NewObjectID(),
		MatchID:   matchID,
		FromID:    fromID,
		ToID:      toID,
		Status:    models.FRStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := friendRequestCol().InsertOne(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

// GetFriendRequestByID fetches a single request by its MongoDB ID.
func GetFriendRequestByID(ctx context.Context, requestID primitive.ObjectID) (*models.FriendRequest, error) {
	req := &models.FriendRequest{}
	err := friendRequestCol().FindOne(ctx, bson.M{"_id": requestID}).Decode(req)
	if err == mongo.ErrNoDocuments {
		return nil, ErrFriendRequestNotFound
	}
	return req, err
}

// ResolveFriendRequest transitions a pending request to accepted or rejected.
// Also creates a Friendship document when accepted.
// Returns the updated FriendRequest and optionally the new Friendship.
func ResolveFriendRequest(ctx context.Context, requestID, resolverID primitive.ObjectID, accept bool) (*models.FriendRequest, *models.Friendship, error) {
	req, err := GetFriendRequestByID(ctx, requestID)
	if err != nil {
		return nil, nil, err
	}

	// Only the receiver can resolve
	if req.ToID != resolverID {
		return nil, nil, ErrNotAuthorized
	}
	if req.Status != models.FRStatusPending {
		return nil, nil, ErrRequestNotPending
	}

	newStatus := models.FRStatusRejected
	if accept {
		newStatus = models.FRStatusAccepted
	}

	now := time.Now().UTC()
	_, err = friendRequestCol().UpdateOne(ctx,
		bson.M{"_id": requestID},
		bson.M{"$set": bson.M{"status": newStatus, "updatedAt": now}},
	)
	if err != nil {
		return nil, nil, err
	}
	req.Status = newStatus
	req.UpdatedAt = now

	// If accepted, create the permanent Friendship record
	if !accept {
		return req, nil, nil
	}

	friendship := &models.Friendship{
		ID:        primitive.NewObjectID(),
		User1ID:   req.FromID,
		User2ID:   req.ToID,
		MatchID:   req.MatchID,
		CreatedAt: now,
	}
	if _, err := friendshipCol().InsertOne(ctx, friendship); err != nil {
		return nil, nil, err
	}
	return req, friendship, nil
}

// GetMyFriendRequests returns all pending requests sent TO the caller.
func GetMyFriendRequests(ctx context.Context, userID primitive.ObjectID) ([]models.FriendRequest, error) {
	cursor, err := friendRequestCol().Find(ctx, bson.M{
		"toId":   userID,
		"status": models.FRStatusPending,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.FriendRequest
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ── Friendships ───────────────────────────────────────────────────────────────

// AreFriends returns true if a Friendship record already exists between the two users.
func AreFriends(ctx context.Context, a, b primitive.ObjectID) (bool, error) {
	count, err := friendshipCol().CountDocuments(ctx, bson.M{
		"$or": bson.A{
			bson.M{"user1Id": a, "user2Id": b},
			bson.M{"user1Id": b, "user2Id": a},
		},
	})
	return count > 0, err
}

// GetMyFriends returns all Friendship documents where the caller is user1 or user2.
func GetMyFriends(ctx context.Context, userID primitive.ObjectID) ([]models.Friendship, error) {
	cursor, err := friendshipCol().Find(ctx, bson.M{
		"$or": bson.A{
			bson.M{"user1Id": userID},
			bson.M{"user2Id": userID},
		},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.Friendship
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
