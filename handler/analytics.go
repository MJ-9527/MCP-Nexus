package handler

import (
	"MCP-Nexus/middleware"
	"MCP-Nexus/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct{ svc *service.AnalyticsService }

func NewAnalyticsHandler(svc *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

func (h *AnalyticsHandler) Overview(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		days = parseDays(d)
		if days < 1 || days > 90 {
			days = 7
		}
	}
	auditEntries := middleware.FlushAuditEntries()
	entries := make([]service.AuditEntry, len(auditEntries))
	for i, e := range auditEntries {
		entries[i] = service.AuditEntry{
			RequestID:  e.RequestID,
			Timestamp:  e.Timestamp,
			Method:     e.Method,
			Path:       e.Path,
			UserID:     e.UserID,
			Username:   e.Username,
			Role:       e.Role,
			StatusCode: e.StatusCode,
			Code:       e.Code,
			Message:    e.Message,
			LatencyMs:  e.LatencyMs,
			ToolID:     e.ToolID,
			ToolName:   e.ToolName,
		}
	}
	h.svc.AddEntries(entries)
	data, err := h.svc.Overview(c.Request.Context(), days)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "stats query failed")
		return
	}
	RespondSuccess(c, data)
}

func parseDays(s string) int {
	var n int
	for _, ch := range s {
		if ch >= 48 && ch <= 57 {
			n = n*10 + int(ch-48)
		}
	}
	return n
}

