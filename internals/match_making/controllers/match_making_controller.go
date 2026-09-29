package controller

import (
	"context"
	"net/http"
	"time"

	"ranchat/internals/match_making/dto"
	service "ranchat/internals/match_making/services"

	"github.com/gin-gonic/gin"
)

type MatchMakingController struct {
	matchService *service.MatchMakingService
}

func NewMatchMakingController(
	matchService *service.MatchMakingService,
) *MatchMakingController {
	return &MatchMakingController{
		matchService: matchService,
	}
}

func (c *MatchMakingController) StartMatching(ctx *gin.Context) {

	var req dto.MatchmakingRequest

	// Parse request body
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "bad request",
		})
		return
	}

	// Get user ID from middleware
	val, exists := ctx.Get("userId")
	if !exists {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "unauthorized",
		})
		return
	}

	userID, ok := val.(string)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid user id",
		})
		return
	}

	// Set user ID from middleware
	req.UserID = userID

	// Create a timeout context
	matchCtx, cancel := context.WithTimeout(
		ctx.Request.Context(),
		30*time.Second,
	)
	defer cancel()

	// Start matchmaking
	ch := c.matchService.MatchAndEnqueue(
		matchCtx,
		req,
	)

	// Wait for match / timeout / client disconnect
	select {

	case matchedRequest := <-ch:

		ctx.JSON(http.StatusOK, gin.H{
			"status": "matched",
			"match":  matchedRequest,
		})

	case <-matchCtx.Done():

		// Remove user from pending matchmaking
		c.matchService.CancelMatching(userID)

		if ctx.Request.Context().Err() != nil {
			// Client disconnected
			return
		}

		// Our 30-second timeout expired
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "retry",
			"message": "No match found, please retry",
		})
	}
}
