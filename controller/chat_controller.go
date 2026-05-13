// chat_controller.go — HTTP + WebSocket handlers for friend chat.
//
// REST endpoints (protected, require JWT):
//   GET    /api/v1/chats                           → list my chats (friend list view)
//   POST   /api/v1/chats/open                      → open/create chat with a friend
//   GET    /api/v1/chats/:chatId/messages           → paginated message history
//   POST   /api/v1/chats/:chatId/messages           → send a message (HTTP fallback)
//   DELETE /api/v1/chats/:chatId/messages/:msgId   → soft-delete a message
//
// WebSocket:
//   GET /ws/chat/:chatId?token=<jwt>               → real-time chat session
package controller

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ranchat_backend/middleware"
	"ranchat_backend/models"
	"ranchat_backend/repository"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

// ── REST handlers ─────────────────────────────────────────────────────────────

// GetMyChats handles GET /api/v1/chats
// Returns all chats for the calling user, sorted by lastMessageAt desc.
func GetMyChats(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	chats, status, err := service.GetMyChats(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "chats fetched", chats)
}

// OpenChat handles POST /api/v1/chats/open
// Body: { "friendId": "<hex userId>" }
// Creates or returns the existing Chat document for this friendship.
func OpenChat(c *gin.Context) {
	callerID := c.GetString(middleware.ContextKeyUserID)

	var body struct {
		FriendID string `json:"friendId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	chat, status, err := service.GetOrOpenChat(c.Request.Context(), callerID, body.FriendID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "chat opened", chat)
}

// GetChatMessages handles GET /api/v1/chats/:chatId/messages
// Query params:
//   limit    — messages per page  (default 50, max 100)
//   beforeId — cursor for older pages (ObjectId hex)
func GetChatMessages(c *gin.Context) {
	callerID := c.GetString(middleware.ContextKeyUserID)
	chatID := c.Param("chatId")

	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	beforeID := c.Query("beforeId")

	msgs, status, err := service.GetChatMessages(c.Request.Context(), callerID, chatID, limit, beforeID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "messages fetched", msgs)
}

// SendMessageHTTP handles POST /api/v1/chats/:chatId/messages
// Body: { "text": "<message content>" }
// HTTP fallback — the WS handler is preferred for real-time delivery.
func SendMessageHTTP(c *gin.Context) {
	callerID := c.GetString(middleware.ContextKeyUserID)
	chatID := c.Param("chatId")

	var body struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	msg, status, err := service.SendChatMessage(c.Request.Context(), callerID, chatID, body.Text)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}
	utils.SendSuccess(c, status, "message sent", msg)
}

// DeleteMessage handles DELETE /api/v1/chats/:chatId/messages/:messageId
// Soft-deletes the message (only the sender can delete their own messages).
func DeleteMessage(c *gin.Context) {
	callerID := c.GetString(middleware.ContextKeyUserID)
	messageID := c.Param("messageId")

	msg, status, err := service.DeleteChatMessage(c.Request.Context(), callerID, messageID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}
	utils.SendSuccess(c, http.StatusOK, "message deleted", msg)
}

// ── WebSocket handler ─────────────────────────────────────────────────────────

// ConnectChatWS handles GET /ws/chat/:chatId?token=<jwt>
//
// Flow:
//  1. Validate JWT from ?token= query param
//  2. Confirm the caller is a member of this chat
//  3. Upgrade HTTP → WebSocket
//  4. Register into the in-memory chat room
//  5. Start write pump (goroutine) + read pump (blocking)
func ConnectChatWS(c *gin.Context) {
	chatID := c.Param("chatId")

	// 1. JWT validation (WS handshake can't set Authorization header easily)
	token := c.Query("token")
	if token == "" {
		utils.SendError(c, http.StatusUnauthorized, "token query param required")
		return
	}
	claims, err := utils.ParseToken(token)
	if err != nil {
		utils.SendError(c, http.StatusUnauthorized, "invalid or expired token")
		return
	}
	userID := claims.UserID

	// 2. Confirm membership — the chat must already exist and caller must be a member
	ctx := c.Request.Context()
	chat, err := repository.GetChatByChatID(ctx, chatID)
	if err != nil {
		utils.SendError(c, http.StatusNotFound, "chat not found")
		return
	}
	memberFound := false
	for _, m := range chat.Members {
		if m == userID {
			memberFound = true
			break
		}
	}
	if !memberFound {
		utils.SendError(c, http.StatusForbidden, "you are not a member of this chat")
		return
	}

	// 3. Upgrade HTTP → WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ChatWS] upgrade error user=%s chat=%s: %v", userID, chatID, err)
		return
	}

	// 4. Register into the chat room
	client := repository.JoinChatRoom(chatID, userID, conn)

	// 5. Write pump (non-blocking)
	go client.WritePump()

	// 6. Read pump (blocks until disconnect)
	client.ReadPump(handleChatWSMessage, func(uid, cid string) {
		log.Printf("[ChatWS] user=%s disconnected from chat=%s", uid, cid)
		repository.LeaveChatRoom(cid, uid)
	})
}

// handleChatWSMessage dispatches incoming chat WebSocket events.
func handleChatWSMessage(client *repository.ChatClient, msgType string, raw []byte) {
	ctx := utils.BackgroundContext()

	switch msgType {

	case models.WSClientEventSendMessage:
		var payload struct {
			Text string `json:"text"`
		}
		if err := utils.ParseJSON(raw, &payload); err != nil || payload.Text == "" {
			repository.SendToChat(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": "text is required"},
			})
			return
		}

		msg, _, err := service.SendChatMessage(ctx, client.UserID, client.ChatID, payload.Text)
		if err != nil {
			repository.SendToChat(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": err.Error()},
			})
			return
		}

		// SendChatMessage already broadcasts to the room; no extra send needed.
		// But confirm back to sender with the persisted messageId.
		_ = msg

	case models.WSClientEventDeleteMessage:
		var payload struct {
			MessageID string `json:"messageId"`
		}
		if err := utils.ParseJSON(raw, &payload); err != nil || payload.MessageID == "" {
			repository.SendToChat(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": "messageId is required"},
			})
			return
		}

		if _, _, err := service.DeleteChatMessage(ctx, client.UserID, payload.MessageID); err != nil {
			repository.SendToChat(client, models.WSMessage{
				Type: "ERROR",
				Data: map[string]string{"message": err.Error()},
			})
			return
		}
		// DeleteChatMessage already broadcasts MESSAGE_DELETED to the room.

	default:
		log.Printf("[ChatWS] unknown event type=%s from user=%s", msgType, client.UserID)
	}
}
