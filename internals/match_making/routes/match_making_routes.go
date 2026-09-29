package routes

import (
	controller "ranchat/internals/match_making/controllers"
	"ranchat/internals/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterMatchMakingRoutes(
	r *gin.Engine,
	controller *controller.MatchMakingController,
) {
	routes := r.Group(
		"/v1/matching",
		middleware.AuthMiddleware(),
	)

	{
		routes.POST("/start", controller.StartMatching)
	}
}
