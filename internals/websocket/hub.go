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
	if previous, exists := h.connections[userID]; exists && previous != conn {
		_ = previous.Close()
	}
	h.connections[userID] = conn
}

// Unregister on disconnect
func (h *WebSocketHub) Unregister(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if current, exists := h.connections[userID]; exists && current == conn {
		_ = current.Close()
		delete(h.connections, userID)
	}
}

// Direct message push to a specific user
func (h *WebSocketHub) SendToUser(userID string, payload interface{}) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	conn, exists := h.connections[userID]
	if !exists {
		return false // Target user is offline
	}

	if err := conn.WriteJSON(payload); err != nil {
		_ = conn.Close()
		delete(h.connections, userID)
		return false
	}
	return true
}
