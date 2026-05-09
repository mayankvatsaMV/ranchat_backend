// match_controller.go — HTTP handlers for match endpoints.
package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ranchat_backend/middleware"
	"ranchat_backend/models"
	"ranchat_backend/service"
	"ranchat_backend/utils"
)

// StartSearch handles POST /api/v1/matches/search
//
// Accepts an optional JSON body with search preferences:
//
//	{ "wantGender": "anyone" | "male" | "female" | "other" }
//
// Omitting the body or wantGender defaults to "anyone" (match anyone).
//
// Long-polling behaviour:
//   - If a compatible partner is already waiting → responds 200 immediately.
//   - If no partner yet → the request HANGS (up to 30s) until one arrives.
//     When User B calls this endpoint, User A's hanging request resolves with 200.
//   - After 30s with no partner → responds 202 so the client can retry.
func StartSearch(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)

	// Parse optional preferences body — ignore bind errors so clients that
	// send no body at all still get the default "anyone" behaviour.
	var req models.StartSearchRequest
	_ = c.ShouldBindJSON(&req)

	// Normalise empty string to "anyone".
	if req.WantGender == "" {
		req.WantGender = models.PrefAnyone
	}

	prefs := models.SearchPreferences{
		WantGender: req.WantGender,
	}

	matchID, partnerID, waitCh := service.EnqueueAndMatch(c.Request.Context(), userID, prefs)

	if waitCh == nil && matchID != "" {
		// Instant match — respond immediately.
		utils.SendSuccess(c, http.StatusOK, "match found", map[string]string{
			"matchId":   matchID,
			"partnerId": partnerID,
		})
		return
	}

	if waitCh == nil {
		// EnqueueAndMatch failed to resolve the user profile — rare error path.
		utils.SendError(c, http.StatusInternalServerError, "failed to start search")
		return
	}

	// No partner yet — block until one arrives, client disconnects, or 30s timeout.
	select {
	case result := <-waitCh:
		// Partner arrived — User A's request resolves right now.
		utils.SendSuccess(c, http.StatusOK, "match found", map[string]string{
			"matchId":   result.MatchID,
			"partnerId": result.PartnerID,
		})

	case <-c.Request.Context().Done():
		// Client disconnected — clean up.
		service.CancelSearch(c.Request.Context(), userID)

	case <-time.After(30 * time.Second):
		// Timeout — tell client to retry.
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
