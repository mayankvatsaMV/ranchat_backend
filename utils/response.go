// response.go — helpers for writing consistent JSON responses from every handler.
// All API responses use the same envelope: { success, message, data? }
package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse is the standard JSON envelope returned by every endpoint.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"` // omitempty means "omit if nil"
}

// SendSuccess writes a successful JSON response (2xx status).
func SendSuccess(c *gin.Context, statusCode int, message string, data interface{}) {
	c.JSON(statusCode, APIResponse{Success: true, Message: message, Data: data})
}

// SendError writes a failed JSON response and stops the handler chain (Abort).
func SendError(c *gin.Context, statusCode int, message string) {
	c.AbortWithStatusJSON(statusCode, APIResponse{Success: false, Message: message})
}

// SendValidationError is a shorthand for a 400 Bad Request caused by bad input.
func SendValidationError(c *gin.Context, err error) {
	SendError(c, http.StatusBadRequest, err.Error())
}
