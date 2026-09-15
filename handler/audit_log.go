package handler

import (
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type AuditLogHandler struct{ service *service.AuditLogService }

func NewAuditLogHandler(s *service.AuditLogService) *AuditLogHandler {
	return &AuditLogHandler{service: s}
}

func (h *AuditLogHandler) Create(c *gin.Context) {
	var req model.CreateAuditLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "审计日志参数无效")
		return
	}
	log, err := h.service.Record(c.Request.Context(), req)
	if errors.Is(err, service.ErrInvalidAuditLog) {
		respondError(c, http.StatusBadRequest, "审计日志参数无效")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "审计日志写入失败")
		return
	}
	respondSuccess(c, log)
}

func (h *AuditLogHandler) List(c *gin.Context) {
	filter := repository.AuditLogFilter{RequestID: c.Query("request_id"), Status: c.Query("status")}
	var err error
	if value, ok := c.GetQuery("user_id"); ok {
		filter.UserID, err = parsePositive(value)
		if err != nil {
			respondError(c, http.StatusBadRequest, "用户 ID 无效")
			return
		}
	}
	if value, ok := c.GetQuery("tool_id"); ok {
		filter.ToolID, err = parsePositive(value)
		if err != nil {
			respondError(c, http.StatusBadRequest, "工具 ID 无效")
			return
		}
	}
	if value, ok := c.GetQuery("limit"); ok {
		filter.Limit, err = strconv.Atoi(value)
		if err != nil || filter.Limit < 0 || filter.Limit > 100 {
			respondError(c, http.StatusBadRequest, "limit 无效")
			return
		}
	}
	if value, ok := c.GetQuery("offset"); ok {
		filter.Offset, err = strconv.Atoi(value)
		if err != nil || filter.Offset < 0 {
			respondError(c, http.StatusBadRequest, "offset 无效")
			return
		}
	}
	logs, total, err := h.service.List(c.Request.Context(), filter)
	if errors.Is(err, service.ErrInvalidAuditLog) {
		respondError(c, http.StatusBadRequest, "审计日志查询参数无效")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "审计日志查询失败")
		return
	}
	respondSuccess(c, gin.H{"items": logs, "total": total, "limit": filter.Limit, "offset": filter.Offset})
}

func parsePositive(value string) (*int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.New("invalid ID")
	}
	return &id, nil
}
