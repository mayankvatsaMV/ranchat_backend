// Package controller handles HTTP — parse the request, call the service, send the response.
package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ranchat_backend/middleware"
	"ranchat_backend/models"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

// AnonymousLogin handles POST /api/v1/auth/anonymous
func AnonymousLogin(c *gin.Context) {
	var req models.AnonymousLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, status, err := service.AnonymousLogin(c.Request.Context(), &req)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	msg := "Login successful"
	if status == http.StatusCreated {
		msg = "Account created"
	}
	utils.SendSuccess(c, status, msg, resp)
}

// GetMe handles GET /api/v1/users/me
func GetMe(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	user, status, err := service.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User fetched", user)
}

// GetUserByID handles GET /api/v1/users/:userId
func GetUserByID(c *gin.Context) {
	userID := c.Param("userId")

	user, status, err := service.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User fetched", user)
}

// UpdateMe handles PATCH /api/v1/users/me
func UpdateMe(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendError(c, http.StatusBadRequest, err.Error())
		return
	}

	user, status, err := service.UpdateUser(c.Request.Context(), userID, &req)
	if err != nil {
		utils.SendError(c, status, err.Error())
		return
	}

	utils.SendSuccess(c, http.StatusOK, "User updated", user)
}
