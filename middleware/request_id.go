package middleware

import (
	"context"

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
		// 同步注入 request context，供 service 层审计日志读取
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "request_id", requestID))
		c.Next()
	}
}
