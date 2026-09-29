package controller

import (
	"net/http"
	"ranchat/internals/authentication/models"
	"ranchat/internals/authentication/service"
	presenceService "ranchat/internals/presence/services"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserController struct {
	Service         *service.UserServices
	PresenceService *presenceService.PresenceService
}

func (u *UserController) SignUpUser(ctx *gin.Context) {
	var user models.User

	// Parse JSON body
	if err := ctx.ShouldBindJSON(&user); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if user.UserId.IsZero() {
		user.UserId = bson.NewObjectID()
	}
	user.CreatedAt = time.Now()
	user.PremiumTill = time.Now().Add(-1 * time.Millisecond)

	// Call service to add user
	token, errResp, err := u.Service.AddUser(ctx, user)
	if err != nil {
		ctx.JSON(errResp.StatusCode, gin.H{"error": errResp.Message})
		return
	}

	// Success response
	ctx.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"token":   token,
	})
}

func (u *UserController) GetUser(ctx *gin.Context) {
	userId, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Convert interface{} to string
	idStr, ok := userId.(string)
	if !ok || idStr == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user identity"})
		return
	}
	user, errResp, err := u.Service.FindUser(ctx, idStr)
	if err != nil {
		ctx.JSON(errResp.StatusCode, gin.H{"error": errResp.Message})
		return
	}

	ctx.JSON(http.StatusOK, user)
}

// func (u *UserController) UpdateUser(ctx *gin.Context) {
// 	userId,exists:=ctx.Get("userId")
// 	if !exists{

// 	}
// }
