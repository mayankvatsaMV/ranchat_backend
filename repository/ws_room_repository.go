// ws_room_repository.go — In-memory store for active WebSocket match rooms.
//
// A "room" represents the live WebSocket session for an active match.
// It is a temporary structure: created when a match starts, deleted when it ends.
//
// Structure:
//   rooms[matchID] = {
//       user1ID: *Client,
//       user2ID: *Client,
//   }
//
// This lets the server instantly deliver events (friend request, accept, etc.)
// directly to the partner's WebSocket connection without any database lookup.
package repository

import (
	"encoding/json"
	"sync"

	"ranchat_backend/models"

	"github.com/gorilla/websocket"
)

// WSClient represents one active WebSocket connection inside a match room.
type WSClient struct {
	UserID  string
	MatchID string
	conn    *websocket.Conn
	send    chan []byte // buffered outbound message queue
}

// roomStore holds all active match rooms in RAM.
type roomStore struct {
	mu    sync.RWMutex
	rooms map[string]map[string]*WSClient // matchID → userID → *WSClient
}

var wsRooms = &roomStore{
	rooms: make(map[string]map[string]*WSClient),
}

// JoinRoom registers a WebSocket client into their match room.
// Called when a matched user upgrades to WebSocket.
func JoinRoom(matchID, userID string, conn *websocket.Conn) *WSClient {
	client := &WSClient{
		UserID:  userID,
		MatchID: matchID,
		conn:    conn,
		send:    make(chan []byte, 64),
	}

	wsRooms.mu.Lock()
	if wsRooms.rooms[matchID] == nil {
		wsRooms.rooms[matchID] = make(map[string]*WSClient)
	}
	wsRooms.rooms[matchID][userID] = client
	wsRooms.mu.Unlock()

	return client
}

// LeaveRoom removes a client from their match room.
// If the room becomes empty, it is deleted.
func LeaveRoom(matchID, userID string) {
	wsRooms.mu.Lock()
	defer wsRooms.mu.Unlock()

	room, ok := wsRooms.rooms[matchID]
	if !ok {
		return
	}

	if c, exists := room[userID]; exists {
		close(c.send)
		delete(room, userID)
	}

	if len(room) == 0 {
		delete(wsRooms.rooms, matchID)
	}
}

// DeleteRoom forcefully removes an entire match room (called when a match ends).
func DeleteRoom(matchID string) {
	wsRooms.mu.Lock()
	defer wsRooms.mu.Unlock()

	room, ok := wsRooms.rooms[matchID]
	if !ok {
		return
	}
	for _, c := range room {
		close(c.send)
	}
	delete(wsRooms.rooms, matchID)
}

// GetPartner returns the other user's WSClient in the same match room.
// Returns nil if the partner is not connected via WebSocket.
func GetPartner(matchID, myUserID string) *WSClient {
	wsRooms.mu.RLock()
	defer wsRooms.mu.RUnlock()

	room, ok := wsRooms.rooms[matchID]
	if !ok {
		return nil
	}
	for uid, client := range room {
		if uid != myUserID {
			return client
		}
	}
	return nil
}

// SendToClient enqueues a WSMessage for delivery to a specific WSClient.
// Safe to call from any goroutine.
func SendToClient(client *WSClient, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case client.send <- data:
	default:
		// buffer full — client is too slow or dead; don't block the caller
	}
}

// ── Pumps ──────────────────────────────────────────────────────────────────────

// WritePump drains the send channel and writes messages to the WebSocket.
// Must run in its own goroutine.
func (c *WSClient) WritePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// ReadPump reads incoming messages from the WebSocket and dispatches them.
// Blocks until the connection closes. onMessage is called for each valid message.
// onDisconnect is called once when the connection is closed.
func (c *WSClient) ReadPump(
	onMessage func(client *WSClient, msgType string, raw []byte),
	onDisconnect func(userID, matchID string),
) {
	defer func() {
		c.conn.Close()
		onDisconnect(c.UserID, c.MatchID)
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return // connection closed or error
		}

		// Peek at the type field to dispatch
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue // ignore malformed messages
		}

		onMessage(c, envelope.Type, raw)
	}
}
