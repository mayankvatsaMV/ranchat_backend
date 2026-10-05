package routes

import (
	"ranchat/internals/authentication/controller"
	"ranchat/internals/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(
	router *gin.Engine,
	userController *controller.UserController,
) {
	// Public endpoints
	public := router.Group("/v1/auth")
	{
		public.POST("/signup", userController.SignUpUser)
		// public.POST("/login", userController.LoginUser)
	}

	// Protected endpoints
	protected := router.Group(
		"/v1/auth",
		middleware.AuthMiddleware(),
	)
	{
		protected.GET(
			"/user",
			userController.GetUser,
		)

		protected.PATCH(
			"/user",
			userController.UpdateUser,
		)
	}
}
