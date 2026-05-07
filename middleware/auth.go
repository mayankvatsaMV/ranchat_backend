// Package middleware contains Gin middleware functions.
// Middleware runs before your handler and can stop the request (e.g. if auth fails)
// or enrich the context (e.g. by injecting the user ID) before passing control on.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ranchat_backend/utils"
)

// Context keys — use these constants instead of raw strings to avoid typos.
const (
	ContextKeyUserID   = "authUserID"
	ContextKeyDeviceID = "authDeviceID"
)

// AuthRequired is a Gin middleware that checks for a valid JWT on every protected route.
//
// Flow:
//  1. Read the "Authorization: Bearer <token>" header.
//  2. Parse and validate the JWT.
//  3. Inject userID and deviceID into the Gin context so handlers can use them.
//  4. Call c.Next() to let the actual handler run.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Step 1 — read the header
		header := c.GetHeader("Authorization")
		if header == "" {
			utils.SendError(c, http.StatusUnauthorized, "authorization header is required")
			c.Abort() // stop here, don't call the handler
			return
		}

		// Step 2 — expect "Bearer <token>"
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			utils.SendError(c, http.StatusUnauthorized, `authorization header must be: Bearer <token>`)
			c.Abort()
			return
		}

		// Step 3 — validate the JWT
		claims, err := utils.ParseToken(parts[1])
		if err != nil {
			utils.SendError(c, http.StatusUnauthorized, "invalid or expired token: "+err.Error())
			c.Abort()
			return
		}

		// Step 4 — inject into context and continue
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyDeviceID, claims.DeviceID)
		c.Next()
	}
}
