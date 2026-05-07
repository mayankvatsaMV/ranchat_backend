// presence.go — the Presence model.
// This stores only the fast-changing, realtime state for each user.
//
// IMPORTANT: This data lives PURELY IN MEMORY (RAM).
// There is NO "presence" collection in MongoDB.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Presence is an in-memory object tracking a user's realtime state.
type Presence struct {
	UserID   primitive.ObjectID `json:"userId"`
	DeviceID string             `json:"deviceId"`

	// LastActiveAt is updated by the client heartbeat every ~20 seconds.
	// We derive IsOnline() from this instead of trusting disconnect events.
	LastActiveAt time.Time `json:"lastActiveAt"`

	// Matchmaking state
	IsSearching    bool                `json:"isSearching"`
	IsMatched      bool                `json:"isMatched"`
	CurrentMatchID *primitive.ObjectID `json:"currentMatchId"` // nil = no active match

	// Chat state
	ActiveChatID *string `json:"activeChatId"` // nil = not in any chat
	TypingTo     *string `json:"typingTo"`     // userId the user is currently typing to; nil = not typing

	UpdatedAt time.Time `json:"updatedAt"`
}

// IsOnline returns true if a heartbeat was received within the last 30 seconds.
func (p *Presence) IsOnline() bool {
	return time.Since(p.LastActiveAt) < 30*time.Second
}
