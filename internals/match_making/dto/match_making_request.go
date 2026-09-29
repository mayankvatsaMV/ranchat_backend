package dto

import "time"

type MatchmakingRequest struct {
	UserID     string    `json:"user_id,omitempty"`
	Interest   string    `json:"interest"`
	Gender     string    `json:"gender"`
	GenderPref string    `json:"gender_pref"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}
