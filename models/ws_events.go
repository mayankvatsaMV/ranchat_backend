// ws_events.go — WebSocket event type constants and JSON envelope types.
// These are ONLY used during active matched sessions.
// Long-polling is used for matchmaking; WebSocket is used ONLY after a match is found.
package models

// WebSocket event type constants (client ↔ server)
const (
	// Server → Client
	WSEventFriendRequestReceived = "FRIEND_REQUEST_RECEIVED" // partner sent you a request
	WSEventFriendRequestAccepted = "FRIEND_REQUEST_ACCEPTED" // your request was accepted
	WSEventFriendRequestRejected = "FRIEND_REQUEST_REJECTED" // your request was rejected
	WSEventPartnerDisconnected   = "PARTNER_DISCONNECTED"    // partner left the match room

	// Client → Server  (client sends these as JSON over the WebSocket)
	WSClientEventSendFriendRequest   = "SEND_FRIEND_REQUEST"
	WSClientEventAcceptFriendRequest = "ACCEPT_FRIEND_REQUEST"
	WSClientEventRejectFriendRequest = "REJECT_FRIEND_REQUEST"
)

// WSMessage is the generic JSON envelope for all WebSocket messages (both directions).
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// WSFriendRequestPayload is sent when a friend request event is delivered over WS.
type WSFriendRequestPayload struct {
	RequestID string `json:"requestId"` // the MongoDB _id of the FriendRequest
	FromID    string `json:"fromId"`
	MatchID   string `json:"matchId"`
}

// WSAcceptPayload is sent when a friend request is accepted.
type WSAcceptPayload struct {
	RequestID    string `json:"requestId"`
	FriendshipID string `json:"friendshipId"`
}
