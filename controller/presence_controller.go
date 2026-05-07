// presence_controller.go — HTTP handlers for presence endpoints.
package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ranchat_backend/middleware"
	"ranchat_backend/models"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

// Heartbeat handles POST /api/v1/presence/heartbeat
func Heartbeat(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	status, err := service.Heartbeat(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "heartbeat received", nil)
}

// GetPresence handles GET /api/v1/presence/:userId
func GetPresence(c *gin.Context) {
	userID := c.Param("userId")

	presence, status, err := service.GetPresence(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "presence fetched", presence)
}

// UpdateMyPresence handles PATCH /api/v1/presence
func UpdateMyPresence(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	var req models.UpdatePresenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	presence, status, err := service.UpdatePresence(c.Request.Context(), userID, &req)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "presence updated", presence)
}
