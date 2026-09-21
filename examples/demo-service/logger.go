package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// requestIDKey 用于在 context 中传递 request_id。
type requestIDKey struct{}

// setupLogger 初始化结构化日志，输出到 w（默认 stderr）。
func setupLogger(w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// withRequestID 把 request_id 注入 context。
func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// requestIDFromContext 从 context 读取 request_id。
func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// logAttr 构建带 request_id 的日志属性。
func logAttr(ctx context.Context, attrs ...slog.Attr) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs)+1)
	if rid := requestIDFromContext(ctx); rid != "" {
		out = append(out, slog.String("request_id", rid))
	}
	out = append(out, attrs...)
	return out
}

// timeNowMS 返回当前毫秒时间戳（用于指标）。
func timeNowMS() int64 {
	return time.Now().UnixMilli()
}

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

// jsonString 把 any 序列化为 JSON 字符串，用于日志摘要。
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
