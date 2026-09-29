package middleware

import (
	"net/http"
	"ranchat/internals/authentication/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			ctx.Abort()
			return
		}

		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header"})
			ctx.Abort()
			return
		}
		tokenStr := parts[1]

		claims, err := utils.VerifyJWTToken(tokenStr)
		if err != nil {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			ctx.Abort()
			return
		}

		// Store userId in context
		ctx.Set("userId", claims.ID)
		ctx.Next()
	}
}
