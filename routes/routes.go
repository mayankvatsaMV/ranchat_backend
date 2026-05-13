// routes.go — wires URL paths to their handler functions.
package routes

import (
	"github.com/gin-gonic/gin"

	"ranchat_backend/controller"
	"ranchat_backend/middleware"
)

// RegisterRoutes attaches all API routes to the Gin engine.
func RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")

	// ── Public (no token needed) ──────────────────────────────────────────────
	v1.POST("/auth/anonymous", controller.AnonymousLogin)

	// ── WebSocket — Active match sessions only (token in query param) ─────────
	// Connect AFTER receiving a matchId from POST /matches/search
	r.GET("/ws/match/:matchId", controller.ConnectMatchWS)

	// ── WebSocket — Friend chat sessions (token in query param) ────────────
	// Connect AFTER calling POST /api/v1/chats/open to get the chatId
	r.GET("/ws/chat/:chatId", controller.ConnectChatWS)

	// ── Protected (valid JWT required) ────────────────────────────────────────
	auth := v1.Group("/")
	auth.Use(middleware.AuthRequired())
	{
		// User profile
		auth.GET("/users/me", controller.GetMe)
		auth.PATCH("/users/me", controller.UpdateMe)
		auth.GET("/users/:userId", controller.GetUserByID)

		// Presence (realtime online state)
		auth.POST("/presence/heartbeat", controller.Heartbeat) // keep-alive ping every ~20s
		auth.PATCH("/presence", controller.UpdateMyPresence)   // set isSearching → triggers matchmaking
		auth.GET("/presence/:userId", controller.GetPresence)

		// Matchmaking (long-polling)
		auth.POST("/matches/search", controller.StartSearch)    // enter queue → instant match or 202 waiting
		auth.DELETE("/matches/search", controller.CancelSearch) // cancel search, leave queue
		auth.GET("/matches/me", controller.GetMyMatch)          // check current match (reconnection)
		auth.DELETE("/matches/me", controller.LeaveMatch)       // leave active match

		// Friends
		auth.POST("/friends/requests", controller.SendFriendRequest)                     // send a request (HTTP fallback)
		auth.GET("/friends/requests", controller.GetMyFriendRequests)                    // list pending received requests
		auth.POST("/friends/requests/:requestId/accept", controller.AcceptFriendRequest) // accept a request
		auth.POST("/friends/requests/:requestId/reject", controller.RejectFriendRequest) // reject a request
		auth.GET("/friends", controller.GetMyFriends)                                    // list all friendships

		// Chat (friend messaging)
		auth.GET("/chats", controller.GetMyChats)                                                   // friend list + chat previews
		auth.POST("/chats/open", controller.OpenChat)                                               // open / create a chat with a friend
		auth.GET("/chats/:chatId/messages", controller.GetChatMessages)                             // paginated message history
		auth.POST("/chats/:chatId/messages", controller.SendMessageHTTP)                            // send a message (HTTP fallback)
		auth.DELETE("/chats/:chatId/messages/:messageId", controller.DeleteMessage)                 // soft-delete a message
	}
}
