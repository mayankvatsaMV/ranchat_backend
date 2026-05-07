package models

// ─── Request types (client → server) ─────────────────────────────────────────

// AnonymousLoginRequest is the body the client sends to POST /api/v1/auth/anonymous.
// The same deviceId always maps to the same user account (register + login in one call).
type AnonymousLoginRequest struct {
	FullName  string   `json:"fullName"  binding:"required"`
	Gender    string   `json:"gender"    binding:"required,oneof=male female other"`
	Age       int      `json:"age"       binding:"required,min=13,max=100"`
	About     string   `json:"about"`
	Interests []string `json:"interests"`
	DeviceID  string   `json:"deviceId"  binding:"required"`
}

// UpdateUserRequest is the body for PATCH /api/v1/users/me.
// All fields are pointers — nil means "not provided, don't change this field".
type UpdateUserRequest struct {
	FullName  *string   `json:"fullName"  binding:"omitempty,min=1"`
	Gender    *string   `json:"gender"    binding:"omitempty,oneof=male female other"`
	Age       *int      `json:"age"       binding:"omitempty,min=13,max=100"`
	About     *string   `json:"about"`
	Interests *[]string `json:"interests"`
}

// UpdatePresenceRequest is the body for PATCH /api/v1/presence.
// Only send the fields you want to change — nil fields are left as-is.
type UpdatePresenceRequest struct {
	IsSearching  *bool   `json:"isSearching"`
	ActiveChatID *string `json:"activeChatId"` // send null JSON to clear the chat
	TypingTo     *string `json:"typingTo"`     // send null JSON to stop typing indicator
}

// ─── Auth response ────────────────────────────────────────────────────────────

// AuthResponse is returned after a successful login.
// User and Presence are the raw models — no separate response types needed.
type AuthResponse struct {
	Token     string    `json:"token"`
	ExpiresAt string    `json:"expiresAt"`
	User      *User     `json:"user"`
	Presence  *Presence `json:"presence"`
}
