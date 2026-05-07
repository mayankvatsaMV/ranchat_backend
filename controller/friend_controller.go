// friend_controller.go — HTTP handlers for the friend request REST endpoints.
// These are fallback / history endpoints. Primary delivery is over WebSocket.
package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ranchat_backend/middleware"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

// SendFriendRequest handles POST /api/v1/friends/requests
// Body: { "matchId": "<matchId>" }
func SendFriendRequest(c *gin.Context) {
	senderID := c.GetString(middleware.ContextKeyUserID)

	var req struct {
		MatchID string `json:"matchId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	friendReq, status, err := service.SendFriendRequest(c.Request.Context(), senderID, req.MatchID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, status, "friend request sent", friendReq)
}

// AcceptFriendRequest handles POST /api/v1/friends/requests/:requestId/accept
func AcceptFriendRequest(c *gin.Context) {
	resolverID := c.GetString(middleware.ContextKeyUserID)
	requestID := c.Param("requestId")

	req, friendship, status, err := service.ResolveFriendRequest(c.Request.Context(), resolverID, requestID, true)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, status, "friend request accepted", gin.H{
		"request":    req,
		"friendship": friendship,
	})
}

// RejectFriendRequest handles POST /api/v1/friends/requests/:requestId/reject
func RejectFriendRequest(c *gin.Context) {
	resolverID := c.GetString(middleware.ContextKeyUserID)
	requestID := c.Param("requestId")

	req, _, status, err := service.ResolveFriendRequest(c.Request.Context(), resolverID, requestID, false)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, status, "friend request rejected", req)
}

// GetMyFriendRequests handles GET /api/v1/friends/requests
// Returns all pending requests received by the caller.
func GetMyFriendRequests(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	requests, status, err := service.GetMyFriendRequests(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "friend requests fetched", requests)
}

// GetMyFriends handles GET /api/v1/friends
// Returns all accepted friendships for the caller.
func GetMyFriends(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	friends, status, err := service.GetMyFriends(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "friends fetched", friends)
}
