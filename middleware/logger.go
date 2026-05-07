// logger.go — a custom request logger for Gin.
// Prints each request in a simple, readable format:
//   2025-01-01T10:00:00Z | POST /api/v1/auth/anonymous | 127.0.0.1 | 201 | 3.2ms
package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger returns a Gin middleware that logs every HTTP request.
func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("%s | %s %s | %s | %d | %s\n",
			p.TimeStamp.Format(time.RFC3339),
			p.Method,
			p.Path,
			p.ClientIP,
			p.StatusCode,
			p.Latency,
		)
	})
}
