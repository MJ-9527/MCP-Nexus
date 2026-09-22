package handler

import (
<<<<<<< HEAD
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

=======
	"MCP-Nexus/repository"
	"MCP-Nexus/service"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type AnalyticsHandler struct{ s *service.AnalyticsService }

func NewAnalyticsHandler(s *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{s: s}
}
func (h *AnalyticsHandler) Overview(c *gin.Context) {
	days := 7
	var e error
	if v := c.Query("days"); v != "" {
		days, e = strconv.Atoi(v)
	}
	if e != nil || days <= 0 || days > 365 {
		respondError(c, 400, "days 必须在 1 到 365 之间")
		return
	}
	v, e := h.s.Overview(c.Request.Context(), days)
	if e != nil {
		respondError(c, 500, "统计概览查询失败")
		return
	}
	respondSuccess(c, v)
}
func (h *AnalyticsHandler) Trends(c *gin.Context) {
	start, e1 := time.Parse(time.RFC3339, c.Query("start_time"))
	end, e2 := time.Parse(time.RFC3339, c.Query("end_time"))
	g := c.DefaultQuery("granularity", "hour")
	var tid *int64
	if raw := c.Query("tool_id"); raw != "" {
		v, e := strconv.ParseInt(raw, 10, 64)
		if e != nil || v <= 0 {
			respondError(c, 400, "tool_id 无效")
			return
		}
		tid = &v
	}
	if e1 != nil || e2 != nil {
		respondError(c, 400, "时间参数必须使用 RFC3339")
		return
	}
	v, e := h.s.Trends(c.Request.Context(), start, end, g, tid)
	if errors.Is(e, service.ErrInvalidAnalyticsQuery) {
		respondError(c, 400, "趋势查询参数无效")
		return
	}
	if e != nil {
		respondError(c, 500, "趋势查询失败")
		return
	}
	respondSuccess(c, gin.H{"items": v, "start_time": start, "end_time": end, "granularity": g})
}
func (h *AnalyticsHandler) ListAlerts(c *gin.Context) {
	page, size := 1, 20
	var e error
	if v := c.Query("page"); v != "" {
		page, e = strconv.Atoi(v)
		if e != nil {
			respondError(c, 400, "page 无效")
			return
		}
	}
	if v := c.Query("page_size"); v != "" {
		size, e = strconv.Atoi(v)
		if e != nil {
			respondError(c, 400, "page_size 无效")
			return
		}
	}
	items, total, e := h.s.ListAlerts(c.Request.Context(), repository.AlertFilter{Status: c.Query("status"), Severity: c.Query("severity"), Page: page, PageSize: size})
	if errors.Is(e, service.ErrInvalidAnalyticsQuery) {
		respondError(c, 400, "告警查询参数无效")
		return
	}
	if e != nil {
		respondError(c, 500, "告警查询失败")
		return
	}
	respondSuccess(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}
func (h *AnalyticsHandler) Acknowledge(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		respondError(c, 400, "告警 ID 无效")
		return
	}
	e = h.s.Acknowledge(c.Request.Context(), id, c.GetInt64("user_id"))
	if errors.Is(e, repository.ErrNotFound) {
		respondError(c, 404, "告警不存在或已确认")
		return
	}
	if e != nil {
		respondError(c, http.StatusInternalServerError, "告警确认失败")
		return
	}
	respondSuccess(c, gin.H{"alert_id": id, "status": "acknowledged"})
}
>>>>>>> origin/pull-request
