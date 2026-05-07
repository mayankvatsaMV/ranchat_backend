// Package models holds the User struct and all request/response types.
// Keeping them in one file means you never have to jump between dto/ and models/.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User is what gets stored in the "users" MongoDB collection.
// This holds STABLE profile data that changes rarely.
// Fast-changing realtime state (online, searching, typing) lives in the
// "presence" collection — see models/presence.go.
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"userId"`
	FullName  string             `bson:"fullName"      json:"fullName"`
	Gender    string             `bson:"gender"        json:"gender"`
	Age       int                `bson:"age"           json:"age"`
	About     string             `bson:"about"         json:"about"`
	Interests []string           `bson:"interests"     json:"interests"`

	// Set by the server on account creation
	DeviceID    string     `bson:"deviceId"    json:"deviceId"`
	LastSeen    time.Time  `bson:"lastSeen"    json:"lastSeen"`
	PremiumTill *time.Time `bson:"premiumTill" json:"premiumTill"` // nil = free user
	CreatedAt   time.Time  `bson:"createdAt"   json:"createdAt"`
	UpdatedAt   time.Time  `bson:"updatedAt"   json:"updatedAt"`
}
