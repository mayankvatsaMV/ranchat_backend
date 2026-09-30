package controllers

import (
	"net/http"
	"time"

	"ranchat/internals/friendship/models"
	"ranchat/internals/friendship/services"
	ws "ranchat/internals/websocket"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type FriendshipController struct {
	Service services.FriendshipService
	WSHub   *ws.WebSocketHub // 👈 Direct WS Hub reference
}

func NewFriendshipController(
	service services.FriendshipService,
	ws *ws.WebSocketHub,
) *FriendshipController {
	return &FriendshipController{
		WSHub:   ws,
		Service: service,
	}
}

// SendFriendRequest
func (fc *FriendshipController) SendFriendRequest(ctx *gin.Context) {

	userIDValue, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "invalid user ID",
		})
		return
	}

	senderID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid user ID",
		})
		return
	}

	var request models.FriendRequest

	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Never trust SenderID from the client.
	request.SenderID = senderID
	request.CreatedAt = time.Now()
	request.UpdatedAt = time.Now()

	friendReqObj, err := fc.Service.SendFriendRequest(
		ctx.Request.Context(),
		request,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 2. Direct WS Push to User B (Receiver)
	receiverIDStr := friendReqObj.ReceiverID.Hex()
	fc.WSHub.SendToUser(receiverIDStr, gin.H{
		"event": "friend_request_received",
		"data": gin.H{
			"sender_id":  senderID.Hex(),
			"request_id": friendReqObj.ID.Hex(),
			"message":    "You received a new friend request!",
		},
	})

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "Friend request sent successfully",
	})
}

// AcceptFriendRequest
func (fc *FriendshipController) AcceptFriendRequest(ctx *gin.Context) {

	requestID := ctx.Param("requestId")

	userIDValue, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "invalid user ID",
		})
		return
	}

	friendship, err := fc.Service.AcceptFriendRequest(
		ctx.Request.Context(),
		requestID,
		userID,
	)

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Friend request accepted",
		"friend":  friendship,
	})
}

// DeclineFriendRequest
func (fc *FriendshipController) DeclineFriendRequest(ctx *gin.Context) {

	requestID := ctx.Param("requestId")

	if _, exists := ctx.Get("userId"); !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	if err := fc.Service.DeclineFriendRequest(
		ctx.Request.Context(),
		requestID,
	); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Friend request declined",
	})
}

// RemoveFriend
func (fc *FriendshipController) RemoveFriend(ctx *gin.Context) {

	friendID := ctx.Param("friendshipID")

	if _, exists := ctx.Get("userId"); !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	if err := fc.Service.RemoveFriend(
		ctx.Request.Context(),
		friendID,
	); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Friend removed successfully",
	})
}

// GetAllFriendRequests
func (fc *FriendshipController) GetAllFriendRequests(ctx *gin.Context) {

	userIDValue, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "invalid user ID",
		})
		return
	}

	requests, err := fc.Service.GetAllFriendRequests(
		ctx.Request.Context(),
		userID,
	)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"requests": requests,
	})
}

// GetAllFriends
func (fc *FriendshipController) GetAllFriends(ctx *gin.Context) {

	userIDValue, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "user not authenticated",
		})
		return
	}

	userID, ok := userIDValue.(string)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "invalid user ID",
		})
		return
	}

	friends, err := fc.Service.GetAllFriends(
		ctx.Request.Context(),
		userID,
	)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"friends": friends,
	})
}
