package models

import (
	"time"
)

type Presence struct {
	UserID   string `json:"userId"`
	DeviceID string `json:"deviceId"`

	// LastActiveAt is updated by the client heartbeat every ~20 seconds.
	// We derive IsOnline() from this instead of trusting disconnect events.
	LastActiveAt time.Time `json:"lastActiveAt"`

	// // Matchmaking state
	// IsSearching    bool           `json:"isSearching"`
	// IsMatched      bool           `json:"isMatched"`
	// CurrentMatchID *bson.ObjectID `json:"currentMatchId"` // nil = no active match

	// Chat state
	ActiveChatID *string `json:"activeChatId"` // nil = not in any chat
	TypingTo     *string `json:"typingTo"`     // userId the user is currently typing to; nil = not typing

	UpdatedAt time.Time `json:"updatedAt"`
}
