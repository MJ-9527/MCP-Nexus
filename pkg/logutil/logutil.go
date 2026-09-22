package logutil

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"time"
)

// requestIDKey 用于在 context 中传递 request_id。
type requestIDKey struct{}

// SetupLogger 初始化结构化日志，输出到 w（默认 stderr）。
func SetupLogger(w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// WithRequestID 把 request_id 注入 context。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestIDFromContext 从 context 读取 request_id。
func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// LogAttr 构建带 request_id 的日志属性。
func LogAttr(ctx context.Context, attrs ...slog.Attr) []slog.Attr {
	out := make([]slog.Attr, 0, len(attrs)+1)
	if rid := RequestIDFromContext(ctx); rid != "" {
		out = append(out, slog.String("request_id", rid))
	}
	out = append(out, attrs...)
	return out
}

// TimeNowMS 返回当前毫秒时间戳。
func TimeNowMS() int64 {
	return time.Now().UnixMilli()
}

// JSONString 把 any 序列化为 JSON 字符串，用于日志摘要。
func JSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
