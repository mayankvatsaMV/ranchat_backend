package routes

import (
	"ranchat/internals/authentication/controller"
	"ranchat/internals/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(router *gin.Engine, userController *controller.UserController) {
	// Public endpoints (no middleware)
	public := router.Group("/v1/auth")
	{
		public.POST("/signup", userController.SignUpUser)
		// public.POST("/login", userController.LoginUser)
	}

	// Protected endpoints (middleware applied)
	protected := router.Group("/v1/auth", middleware.AuthMiddleware())
	{
		protected.GET("/user", userController.GetUser)
		// protected.PUT("/user/", userController.UpdateUser)
	}
}
