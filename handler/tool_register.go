package handler

import (
	"MCP-Nexus/model"
	"MCP-Nexus/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ToolRegisterHandler struct{ service *service.ToolService }

func NewToolRegisterHandler(s *service.ToolService) *ToolRegisterHandler {
	return &ToolRegisterHandler{service: s}
}

func (h *ToolRegisterHandler) RegisterTool(c *gin.Context) {
	var req model.RegisterToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "请求参数无效")
		return
	}
	tool, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidTool):
			RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具参数无效")
		case errors.Is(err, service.ErrServerNotFound):
			RespondError(c, http.StatusNotFound, "SERVER_NOT_FOUND", "所属 MCP Server 不存在")
		case errors.Is(err, service.ErrToolExists):
			RespondError(c, http.StatusConflict, "CONFLICT", "同一 Server 下工具名称已存在")
		default:
			RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "工具注册失败")
		}
		return
	}
	RespondSuccess(c, tool)
}
