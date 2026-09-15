package handler

import (
	"net/http"
	"strconv"

	"MCP-Nexus/repository"

	"github.com/gin-gonic/gin"
)

// AnalyticsHandler 统计与审计接口
type AnalyticsHandler struct {
	ch *repository.ClickHouseAuditRepository
}

func NewAnalyticsHandler(ch *repository.ClickHouseAuditRepository) *AnalyticsHandler {
	return &AnalyticsHandler{ch: ch}
}

// Overview GET /api/analytics/overview
func (h *AnalyticsHandler) Overview(c *gin.Context) {
	if h.ch == nil {
		respondErrorCode(c, http.StatusServiceUnavailable, "ANALYTICS_UNAVAILABLE", "ClickHouse 未连接")
		return
	}
	o, err := h.ch.Overview(c.Request.Context())
	if err != nil {
		respondErrorCode(c, http.StatusInternalServerError, "ANALYTICS_ERROR", err.Error())
		return
	}
	respondSuccess(c, o)
}

// AuditLogs GET /api/audit/logs?tool_name=&role=&success=&since_hours=&page=&page_size=
func (h *AnalyticsHandler) AuditLogs(c *gin.Context) {
	if h.ch == nil {
		respondErrorCode(c, http.StatusServiceUnavailable, "AUDIT_UNAVAILABLE", "ClickHouse 未连接")
		return
	}
	f := repository.AuditLogFilter{
		ToolName:   c.Query("tool_name"),
		Role:       c.Query("role"),
		SinceHours: atoiDefault(c.Query("since_hours"), 0),
		Page:       atoiDefault(c.Query("page"), 1),
		PageSize:   atoiDefault(c.Query("page_size"), 20),
	}
	if s := c.Query("success"); s != "" {
		if s == "true" || s == "1" {
			t := true
			f.Success = &t
		} else if s == "false" || s == "0" {
			fl := false
			f.Success = &fl
		}
	}
	logs, total, err := h.ch.QueryLogs(c.Request.Context(), f)
	if err != nil {
		respondErrorCode(c, http.StatusInternalServerError, "AUDIT_QUERY_ERROR", err.Error())
		return
	}
	totalPages := int(total) / f.PageSize
	if int(total)%f.PageSize > 0 {
		totalPages++
	}
	respondSuccess(c, gin.H{
		"items":       logs,
		"page":        f.Page,
		"page_size":   f.PageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
