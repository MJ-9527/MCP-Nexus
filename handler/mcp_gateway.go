package handler

import (
	"MCP-Nexus/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MCPGatewayHandler struct{ service *service.GatewayService }

func NewMCPGatewayHandler(s *service.GatewayService) *MCPGatewayHandler {
	return &MCPGatewayHandler{service: s}
}

func (h *MCPGatewayHandler) ListTools(c *gin.Context) {
	tools, err := h.service.ListTools(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "工具发现失败")
		return
	}
	out := make([]gin.H, 0, len(tools))
	for _, t := range tools {
		out = append(out, gin.H{
			"id":            t.ID,
			"name":          t.Name,
			"description":   t.Description,
			"category":      t.Category,
			"version":       t.Version,
			"health_status": t.HealthStatus,
			"call_count":    t.CallCount,
		})
	}
	RespondSuccess(c, gin.H{"tools": out, "total": len(out)})
}

func (h *MCPGatewayHandler) CallTool(c *gin.Context) {
	toolName := c.Param("toolName")
	if toolName == "" {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具名称不能为空")
		return
	}
	var args map[string]any
	if err := c.ShouldBindJSON(&args); err != nil {
		args = map[string]any{}
	}
	result, err := h.service.CallTool(c.Request.Context(), toolName, args)
	if err != nil {
		switch {
		case err == service.ErrGatewayInternal:
			RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "网关内部错误")
		default:
			RespondError(c, http.StatusServiceUnavailable, "TOOL_OFFLINE", err.Error())
		}
		return
	}
	RespondSuccess(c, result)
}
