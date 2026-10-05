package routers

import (
	"ranchat/internals/chats/controller"

	"github.com/gin-gonic/gin"
)

func RegisterChatRoutes(
	router *gin.RouterGroup,
	controller *controller.ChatController,
	authMiddleware gin.HandlerFunc,
) {
	chat := router.Group("/chat")
	chat.Use(authMiddleware)

	chat.POST("/messages", controller.SendMessage)
	chat.GET("/messages/:receiver_id", controller.GetChatHistory)
	chat.PATCH("/messages/:receiver_id/read", controller.MarkMessagesAsRead)
}
