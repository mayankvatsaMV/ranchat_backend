package routes

import (
	"ranchat/internals/middleware"
	"ranchat/internals/presence/controller"

	"github.com/gin-gonic/gin"
)

func RegisterPresenceRoutes(router *gin.Engine, presenceController *controller.PresenceController) {
	private := router.Group("/v1/presence", middleware.AuthMiddleware())
	{
		// POST /v1/presence → calls controller.UpsertPresence
		private.POST("", presenceController.UpsertPresence)
		private.POST("heartbeat", presenceController.HeartBeat)
		private.PATCH("update", presenceController.UpdatePresence)
	}
}
