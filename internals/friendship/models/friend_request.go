package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type FriendRequest struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	SenderID   bson.ObjectID `bson:"sender_id" json:"sender_id"`
	ReceiverID bson.ObjectID `bson:"receiver_id" json:"receiver_id"`
	Status     string        `bson:"status" json:"status"` // "pending", "accepted", "declined"
	CreatedAt  time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt  time.Time     `bson:"updated_at" json:"updated_at"`
}
