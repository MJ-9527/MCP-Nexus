package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

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
	if value, ok := c.GetQuery("server_id"); ok {
		filter.ServerID, err = parsePositive(value)
		if err != nil {
			respondError(c, http.StatusBadRequest, "Server ID 无效")
			return
		}
	}
	if value, ok := c.GetQuery("start_time"); ok {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			respondError(c, http.StatusBadRequest, "start_time 无效")
			return
		}
		filter.StartTime = &parsed
	}
	if value, ok := c.GetQuery("end_time"); ok {
		parsed, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			respondError(c, http.StatusBadRequest, "end_time 无效")
			return
		}
		filter.EndTime = &parsed
	}
	if value, ok := c.GetQuery("page"); ok {
		filter.Page, err = strconv.Atoi(value)
		if err != nil || filter.Page <= 0 {
			respondError(c, http.StatusBadRequest, "page 无效")
			return
		}
	}
	if value, ok := c.GetQuery("page_size"); ok {
		filter.PageSize, err = strconv.Atoi(value)
		if err != nil || filter.PageSize <= 0 || filter.PageSize > 100 {
			respondError(c, http.StatusBadRequest, "page_size 无效")
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
	page, pageSize := filter.Page, filter.PageSize
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}
	respondSuccess(c, gin.H{"items": logs, "total": total, "page": page, "page_size": pageSize})
}

func parsePositive(value string) (*int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return nil, errors.New("invalid ID")
	}
	return &id, nil
}
