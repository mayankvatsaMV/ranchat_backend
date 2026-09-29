package models

import "time"

type Match struct {
	MatchID   string    `json:"match_id"`
	UserIDs   [2]string `json:"user_ids"`
	CreatedAt time.Time `json:"created_at"`
}
