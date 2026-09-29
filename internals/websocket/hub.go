package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

type WebSocketHub struct {
	mu          sync.RWMutex
	connections map[string]*websocket.Conn
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		connections: make(map[string]*websocket.Conn),
	}
}

// Register user connection on /v1/ws connect
func (h *WebSocketHub) Register(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[userID] = conn
}

// Unregister on disconnect
func (h *WebSocketHub) Unregister(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conn, exists := h.connections[userID]; exists {
		conn.Close()
		delete(h.connections, userID)
	}
}

// Direct message push to a specific user
func (h *WebSocketHub) SendToUser(userID string, payload interface{}) bool {
	h.mu.RLock()
	conn, exists := h.connections[userID]
	h.mu.RUnlock()

	if !exists {
		return false // Target user is offline
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if err := conn.WriteJSON(payload); err != nil {
		conn.Close()
		delete(h.connections, userID)
		return false
	}
	return true
}
