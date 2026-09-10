package handler

import (
	"MCP-Nexus/repository"
	"MCP-Nexus/service"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type ToolQueryHandler struct{ service *service.ToolService }

func NewToolQueryHandler(s *service.ToolService) *ToolQueryHandler {
	return &ToolQueryHandler{service: s}
}

func (h *ToolQueryHandler) ListTools(c *gin.Context) {
	filter, err := parseToolFilter(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "工具查询参数无效")
		return
	}
	tools, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "工具查询失败")
		return
	}
	respondSuccess(c, gin.H{"items": tools, "total": len(tools)})
}

func (h *ToolQueryHandler) GetTool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return
	}
	tool, err := h.service.GetByID(c.Request.Context(), id)
	if errors.Is(err, service.ErrToolNotFound) {
		respondError(c, http.StatusNotFound, "工具不存在")
		return
	}
	if errors.Is(err, service.ErrInvalidTool) {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "工具查询失败")
		return
	}
	respondSuccess(c, tool)
}

func parseToolFilter(c *gin.Context) (repository.ToolFilter, error) {
	filter := repository.ToolFilter{Name: c.Query("q"), Category: c.Query("category"), HealthStatus: c.Query("health_status")}
	if value, ok := c.GetQuery("server_id"); ok {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return filter, errors.New("invalid server_id")
		}
		filter.ServerID = &id
	}
	if value, ok := c.GetQuery("published"); ok {
		published, err := strconv.ParseBool(value)
		if err != nil {
			return filter, err
		}
		filter.Published = &published
	}
	return filter, nil
}
