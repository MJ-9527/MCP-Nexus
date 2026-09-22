package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestIDMiddleware 为每个请求注入/读取 request_id，并放入 context。
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set(requestIDKey{}, rid)
		c.Header("X-Request-ID", rid)
		c.Request = c.Request.WithContext(withRequestID(c.Request.Context(), rid))
		c.Next()
	}
}

// structuredLogger 记录每个请求的结构化访问日志（含 request_id、状态码、耗时、响应大小）。
func structuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rw := newResponseWriter(c.Writer)
		c.Writer = rw
		c.Next()
		metrics.recordRequest(rw.statusCode)

		logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "http request", logAttr(c.Request.Context(),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int64("response_size", rw.size),
		)...)
	}
}

// apiKeyAuth 是敏感工具的鉴权中间件。
// 校验 X-API-Key Header；缺失或不匹配时返回 401 并中止，不会进入下游处理器。
func apiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key != serviceAPIKey {
			metrics.recordError("delete_customer_unauthorized")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer unauthorized", logAttr(c.Request.Context(), slog.Bool("has_key", key != ""))...)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
