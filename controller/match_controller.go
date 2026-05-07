// match_controller.go — HTTP handlers for match endpoints.
package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ranchat_backend/middleware"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

// StartSearch handles POST /api/v1/matches/search
//
// Long-polling endpoint:
//   - If a partner is already waiting → responds 200 immediately with match data.
//   - If no partner yet → the request HANGS (up to 60s) until a partner arrives.
//     When User B calls this endpoint, User A's hanging request resolves with 200.
//   - After 60s with no partner → responds 202 so the client can retry.
func StartSearch(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	matchID, partnerID, waitCh := service.EnqueueAndMatch(c.Request.Context(), userID)

	if waitCh == nil {
		// Instant match — respond immediately
		utils.SendSuccess(c, http.StatusOK, "match found", map[string]string{
			"matchId":   matchID,
			"partnerId": partnerID,
		})
		return
	}

	// No partner yet — block until one arrives, client disconnects, or 60s timeout
	select {
	case result := <-waitCh:
		// Partner arrived — User A's request resolves right now
		utils.SendSuccess(c, http.StatusOK, "match found", map[string]string{
			"matchId":   result.MatchID,
			"partnerId": result.PartnerID,
		})

	case <-c.Request.Context().Done():
		// Client disconnected — clean up
		service.CancelSearch(c.Request.Context(), userID)

	case <-time.After(30 * time.Second):
		// Timeout — tell client to retry
		service.CancelSearch(c.Request.Context(), userID)
		utils.SendSuccess(c, http.StatusAccepted, "still searching... call again to keep waiting", nil)
	}
}

// CancelSearch handles DELETE /api/v1/matches/search
// Removes the user from the matchmaking queue.
func CancelSearch(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	service.CancelSearch(c.Request.Context(), userID)
	utils.SendSuccess(c, http.StatusOK, "search cancelled", nil)
}

// GetMyMatch handles GET /api/v1/matches/me
// Returns the current active match.
// Use this on app restart to resume an interrupted match session.
func GetMyMatch(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	match := service.GetMyMatch(userID)
	if match == nil {
		utils.SendError(c, http.StatusNotFound, "no active match")
		return
	}

	utils.SendSuccess(c, http.StatusOK, "match found", match)
}

// LeaveMatch handles DELETE /api/v1/matches/me
// Ends the current match and notifies the partner via WebSocket.
func LeaveMatch(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	if err := service.EndMatch(c.Request.Context(), userID); err != nil {
		utils.SendError(c, http.StatusNotFound, "no active match to leave")
		return
	}

	utils.SendSuccess(c, http.StatusOK, "match ended", nil)
}

