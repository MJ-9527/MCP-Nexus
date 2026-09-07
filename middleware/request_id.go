package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("request_id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("request_id", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}
