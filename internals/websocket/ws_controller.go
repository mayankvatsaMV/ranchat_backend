package ws

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHandler struct {
	Hub *WebSocketHub
}

func (h *WSHandler) ConnectWS(ctx *gin.Context) {
	userIDValue, exists := ctx.Get("userId")
	if !exists {
		userIDValue, exists = ctx.Get("user_id")
	}
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	// Store connection in Hub
	h.Hub.Register(userID, conn)

	// Keep connection alive & listen for disconnects
	go func() {
		defer h.Hub.Unregister(userID, conn)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}
