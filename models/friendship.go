// friendship.go — MongoDB-persisted models for the friend request system.
// friend_requests  → pending/accepted/rejected requests
// friendships      → permanent accepted connections between two users
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FriendRequestStatus is the lifecycle state of a friend request.
type FriendRequestStatus string

const (
	FRStatusPending  FriendRequestStatus = "pending"
	FRStatusAccepted FriendRequestStatus = "accepted"
	FRStatusRejected FriendRequestStatus = "rejected"
)

// FriendRequest is stored in the "friend_requests" MongoDB collection.
// It is created when User A sends a request to User B during an active match.
type FriendRequest struct {
	ID        primitive.ObjectID  `bson:"_id,omitempty" json:"requestId"`
	MatchID   string              `bson:"matchId"       json:"matchId"`   // the match during which it was sent
	FromID    primitive.ObjectID  `bson:"fromId"        json:"fromId"`    // sender
	ToID      primitive.ObjectID  `bson:"toId"          json:"toId"`      // receiver
	Status    FriendRequestStatus `bson:"status"        json:"status"`
	CreatedAt time.Time           `bson:"createdAt"     json:"createdAt"`
	UpdatedAt time.Time           `bson:"updatedAt"     json:"updatedAt"`
}

// Friendship is stored in the "friendships" MongoDB collection.
// It is created when a FriendRequest is accepted.
type Friendship struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"friendshipId"`
	User1ID   primitive.ObjectID `bson:"user1Id"       json:"user1Id"`
	User2ID   primitive.ObjectID `bson:"user2Id"       json:"user2Id"`
	MatchID   string             `bson:"matchId"       json:"matchId"` // the match that connected them
	CreatedAt time.Time          `bson:"createdAt"     json:"createdAt"`
}
