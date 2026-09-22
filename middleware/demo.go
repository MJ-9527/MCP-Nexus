package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"MCP-Nexus/pkg/logutil"
	"MCP-Nexus/pkg/metrics"
	"MCP-Nexus/pkg/security"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// responseWriter 包装 gin.ResponseWriter 以捕获状态码和响应大小。
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
	size       int64
}

func newResponseWriter(w gin.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)
	return n, err
}

// WriteString implements gin.ResponseWriter.
func (rw *responseWriter) WriteString(s string) (int, error) {
	n, err := rw.ResponseWriter.WriteString(s)
	rw.size += int64(n)
	return n, err
}

// DemoRequestID 为每个请求注入/读取 request_id，并放入 context。
func DemoRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Header("X-Request-ID", rid)
		c.Set("request_id", rid)
		c.Request = c.Request.WithContext(logutil.WithRequestID(c.Request.Context(), rid))
		c.Next()
	}
}

// DemoLogger 记录每个请求的结构化访问日志（含 request_id、状态码、耗时、响应大小）。
func DemoLogger(logger *slog.Logger, m *metrics.ServiceMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rw := newResponseWriter(c.Writer)
		c.Writer = rw
		c.Next()

		if m != nil {
			m.RecordRequest(rw.statusCode)
		}
		if logger != nil {
			logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "http request", logutil.LogAttr(c.Request.Context(),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.Int("status", rw.statusCode),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.Int64("response_size", rw.size),
			)...)
		}
	}
}

// DemoAPIKeyAuth 是敏感工具的鉴权中间件。
// 校验 X-API-Key Header；缺失或不匹配时返回 401 并中止，不会进入下游处理器。
func DemoAPIKeyAuth(logger *slog.Logger, m *metrics.ServiceMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key != security.ServiceAPIKey {
			if m != nil {
				m.RecordError("delete_customer_unauthorized")
			}
			if logger != nil {
				logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer unauthorized", logutil.LogAttr(c.Request.Context(), slog.Bool("has_key", key != ""))...)
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
