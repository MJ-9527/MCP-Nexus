package middleware

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type AuditRecord struct {
	RequestID  string    `json:"request_id"`
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	UserID     int64     `json:"user_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	Role       string    `json:"role,omitempty"`
	StatusCode int       `json:"status_code"`
	Code       string    `json:"code"`
	Message    string    `json:"message,omitempty"`
	LatencyMs  int64     `json:"latency_ms"`
	ToolID     int64     `json:"tool_id,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
}

type auditStore struct {
	entries []AuditRecord
	mu      sync.RWMutex
}

var globalAudit = &auditStore{}

func Audit(callback func(AuditRecord)) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		record := AuditRecord{
			RequestID:  c.GetString("request_id"),
			Timestamp:  start,
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
			StatusCode: c.Writer.Status(),
			LatencyMs:  latency.Milliseconds(),
		}
		if uid, ok := GetCurrentUserID(c); ok {
			record.UserID = uid
		}
		record.Username = GetCurrentUsername(c)
		record.Role = GetCurrentRole(c)
		if code, exists := c.Get("resp_code"); exists {
			record.Code = code.(string)
		}
		if msg, exists := c.Get("resp_message"); exists {
			record.Message = msg.(string)
		}
		if tid, ok := c.Get("tool_id"); ok {
			if id, ok := tid.(int64); ok {
				record.ToolID = id
			}
		}
		if tname, ok := c.Get("tool_name"); ok {
			if name, ok := tname.(string); ok {
				record.ToolName = name
			}
		}
		globalAudit.mu.Lock()
		globalAudit.entries = append(globalAudit.entries, record)
		globalAudit.mu.Unlock()
		if callback != nil {
			callback(record)
		} else {
			payload, _ := json.Marshal(record)
			println(string(payload))
		}
	}
}

func GetAuditEntries() []AuditRecord {
	globalAudit.mu.RLock()
	defer globalAudit.mu.RUnlock()
	out := make([]AuditRecord, len(globalAudit.entries))
	copy(out, globalAudit.entries)
	return out
}

func FlushAuditEntries() []AuditRecord {
	return GetAuditEntries()
}

func SetResponseMeta(c *gin.Context, code, message string) {
	c.Set("resp_code", code)
	c.Set("resp_message", message)
}

