package routes

import (
	"ranchat/internals/friendship/controllers"
	"ranchat/internals/middleware"

	"github.com/gin-gonic/gin"
)

func FriendshipRoutes(
	router *gin.Engine,
	controller controllers.FriendshipController,
) {
	private := router.Group(
		"/v1/friendship",
		middleware.AuthMiddleware(),
	)

	{
		// Send a friend request
		private.POST(
			"/requests",
			controller.SendFriendRequest,
		)

		// Accept a friend request
		private.POST(
			"/requests/:requestId/accept",
			controller.AcceptFriendRequest,
		)

		// Decline a friend request
		private.POST(
			"/requests/:requestId/decline",
			controller.DeclineFriendRequest,
		)

		// Get all incoming friend requests
		private.GET(
			"/requests",
			controller.GetAllFriendRequests,
		)

		// Get all friends
		private.GET(
			"/friends",
			controller.GetAllFriends,
		)

		// Remove a friend
		private.DELETE(
			"/friends/:friendshipID",
			controller.RemoveFriend,
		)
	}
}
