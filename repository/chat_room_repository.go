// chat_room_repository.go — In-memory store for active friend chat WebSocket rooms.
//
// Architecture mirrors ws_room_repository.go but uses chatId as the room key
// and reuses the same WSClient / WritePump / ReadPump machinery.
//
// Structure:
//   chatRooms[chatId] = {
//       userAID: *ChatClient,
//       userBID: *ChatClient,
//   }
//
// A chat room is persistent (remains in memory while at least one member is
// connected), unlike a match room which is torn down when the match ends.
package repository

import (
	"encoding/json"
	"log"
	"sync"

	"ranchat_backend/models"

	"github.com/gorilla/websocket"
)

// ChatClient represents one active WebSocket connection inside a chat room.
type ChatClient struct {
	UserID string
	ChatID string
	conn   *websocket.Conn
	send   chan []byte // buffered outbound message queue
}

// chatRoomStore holds all active friend chat rooms in RAM.
type chatRoomStore struct {
	mu    sync.RWMutex
	rooms map[string]map[string]*ChatClient // chatID → userID → *ChatClient
}

var chatRooms = &chatRoomStore{
	rooms: make(map[string]map[string]*ChatClient),
}

// JoinChatRoom registers a WebSocket client into the chat room for chatID.
func JoinChatRoom(chatID, userID string, conn *websocket.Conn) *ChatClient {
	client := &ChatClient{
		UserID: userID,
		ChatID: chatID,
		conn:   conn,
		send:   make(chan []byte, 128),
	}

	chatRooms.mu.Lock()
	if chatRooms.rooms[chatID] == nil {
		chatRooms.rooms[chatID] = make(map[string]*ChatClient)
	}
	chatRooms.rooms[chatID][userID] = client
	chatRooms.mu.Unlock()

	log.Printf("[ChatWS] user=%s joined chat room=%s", userID, chatID)
	return client
}

// LeaveChatRoom removes a client from the chat room.
// The room itself is kept alive (unlike match rooms) because the friendship persists.
func LeaveChatRoom(chatID, userID string) {
	chatRooms.mu.Lock()
	defer chatRooms.mu.Unlock()

	room, ok := chatRooms.rooms[chatID]
	if !ok {
		return
	}
	if c, exists := room[userID]; exists {
		close(c.send)
		delete(room, userID)
	}
	// Room entry remains so next connection can simply add to it.
	// GC happens when both members leave.
	if len(room) == 0 {
		delete(chatRooms.rooms, chatID)
	}

	log.Printf("[ChatWS] user=%s left chat room=%s", userID, chatID)
}

// GetChatPartner returns the other user's ChatClient in the room, or nil.
func GetChatPartner(chatID, myUserID string) *ChatClient {
	chatRooms.mu.RLock()
	defer chatRooms.mu.RUnlock()

	room, ok := chatRooms.rooms[chatID]
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

// SendToChat enqueues a WSMessage for delivery to a specific ChatClient.
func SendToChat(client *ChatClient, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case client.send <- data:
	default:
		// Buffer full — client is too slow or dead; don't block
	}
}

// BroadcastToChat sends a WSMessage to ALL members of a chat room.
func BroadcastToChat(chatID string, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	chatRooms.mu.RLock()
	defer chatRooms.mu.RUnlock()

	room, ok := chatRooms.rooms[chatID]
	if !ok {
		return
	}
	for _, client := range room {
		select {
		case client.send <- data:
		default:
		}
	}
}

// ── Pumps ─────────────────────────────────────────────────────────────────────

// WritePump drains the send channel and writes messages to the WebSocket.
// Must run in its own goroutine.
func (c *ChatClient) WritePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// ReadPump reads incoming messages and dispatches them via onMessage.
// Blocks until the connection closes; calls onDisconnect once on exit.
func (c *ChatClient) ReadPump(
	onMessage func(client *ChatClient, msgType string, raw []byte),
	onDisconnect func(userID, chatID string),
) {
	defer func() {
		c.conn.Close()
		onDisconnect(c.UserID, c.ChatID)
	}()

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		onMessage(c, envelope.Type, raw)
	}
}
