package models

import "time"

// Match represents an active pairing between two users.
// Lives in RAM only — no MongoDB write for active sessions.
type Match struct {
	MatchID   string    `json:"matchId"`
	User1ID   string    `json:"user1Id"` // first user to enter queue
	User2ID   string    `json:"user2Id"` // second user (triggered the match)
	StartedAt time.Time `json:"startedAt"`
}


