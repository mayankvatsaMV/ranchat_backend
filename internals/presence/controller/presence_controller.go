package controller

import (
	"net/http"
	"ranchat/errors"
	"ranchat/internals/presence/dto"
	service "ranchat/internals/presence/services"

	"github.com/gin-gonic/gin"
)

type PresenceController struct {
	Serv *service.PresenceService
}

func NewPresenceController(serv *service.PresenceService) *PresenceController {
	return &PresenceController{Serv: serv}
}

func (c *PresenceController) GetPresence(ctx *gin.Context) {
	if _, exists := ctx.Get("userId"); !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID := ctx.Param("userId")
	if userID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	presence, err := c.Serv.GetPresence(ctx, userID)
	if err != nil {
		switch err {
		case errors.ErrPresenceNotFound:
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Presence not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"lastActiveAt": presence.LastActiveAt})
}

func (c *PresenceController) UpsertPresence(ctx *gin.Context) {
	uid, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userId := uid.(string)

	var reqBody map[string]string
	if err := ctx.ShouldBindJSON(&reqBody); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	deviceID, ok := reqBody["deviceId"]
	if !ok || deviceID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "deviceId is required"})
		return
	}

	p, err := c.Serv.UpsertPresence(ctx, userId, deviceID)
	if err != nil {
		// Check for sentinel errors
		switch err {
		case errors.ErrPresenceExists:
			ctx.JSON(http.StatusConflict, gin.H{"error": "Presence already exists"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Success response
	ctx.JSON(http.StatusCreated, gin.H{
		"message":  "Presence created successfully",
		"presence": p,
	})
}

func (c *PresenceController) HeartBeat(ctx *gin.Context) {
	uid, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userId := uid.(string)

	// Call service
	if err := c.Serv.HeartBeat(ctx, userId); err != nil {
		switch err {
		case errors.ErrPresenceNotFound:
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Presence not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// Success response
	ctx.JSON(http.StatusOK, gin.H{
		"message": "Heartbeat updated successfully",
		"userId":  userId,
	})
}

func (c *PresenceController) UpdatePresence(ctx *gin.Context) {
	var updateBody dto.UpdatePresenceFields
	if err := ctx.ShouldBindJSON(&updateBody); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userId := uid.(string)

	p, err := c.Serv.UpdatePresence(ctx, userId, updateBody)
	if err != nil {
		switch err {
		case errors.ErrPresenceNotFound:
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Presence not found"})
		default:
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Presence updated successfully",
		"data":    p,
	})
}
