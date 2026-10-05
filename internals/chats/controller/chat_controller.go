package controller

import (
	"net/http"
	"strconv"

	"ranchat/internals/chats/dto"
	"ranchat/internals/chats/services"
	ws "ranchat/internals/websocket"

	"github.com/gin-gonic/gin"
)

type ChatController struct {
	chatService *services.ChatService
	wsHub       *ws.WebSocketHub
}

func NewChatController(chatService *services.ChatService, wsHub *ws.WebSocketHub) *ChatController {
	return &ChatController{
		chatService: chatService,
		wsHub:       wsHub,
	}
}

// SendMessage handles HTTP message sending, persists message to DB, and delivers real-time WS push.
func (cc *ChatController) SendMessage(c *gin.Context) {
	var req dto.SendMessageRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	userIDValue, exists := c.Get("userId")
	if !exists {
		userIDValue, exists = c.Get("user_id")
	}
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	senderID, ok := userIDValue.(string)
	if !ok || senderID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid user id",
		})
		return
	}

	message, err := cc.chatService.SendMessage(
		c.Request.Context(),
		senderID,
		&req,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Push real-time event to receiver via WebSocket if online
	if cc.wsHub != nil {
		cc.wsHub.SendToUser(req.ReceiverID, gin.H{
			"event": "new_message",
			"data":  message,
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": message,
	})
}

// GetChatHistory returns messages between authenticated user and receiver.
func (cc *ChatController) GetChatHistory(c *gin.Context) {
	receiverID := c.Param("receiver_id")

	if receiverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "receiver_id is required",
		})
		return
	}

	userIDValue, exists := c.Get("userId")
	if !exists {
		userIDValue, exists = c.Get("user_id")
	}
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid user id",
		})
		return
	}

	limit := int64(50)

	if limitParam := c.Query("limit"); limitParam != "" {
		parsedLimit, err := strconv.ParseInt(limitParam, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid limit",
			})
			return
		}

		limit = parsedLimit
	}

	messages, err := cc.chatService.GetChatHistory(
		c.Request.Context(),
		userID,
		receiverID,
		limit,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
	})
}

// MarkMessagesAsRead marks messages received from the receiver as read.
func (cc *ChatController) MarkMessagesAsRead(c *gin.Context) {
	receiverID := c.Param("receiver_id")

	if receiverID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "receiver_id is required",
		})
		return
	}

	userIDValue, exists := c.Get("userId")
	if !exists {
		userIDValue, exists = c.Get("user_id")
	}
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid user id",
		})
		return
	}

	if err := cc.chatService.MarkMessagesAsRead(
		c.Request.Context(),
		userID,
		receiverID,
	); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "messages marked as read",
	})
}
