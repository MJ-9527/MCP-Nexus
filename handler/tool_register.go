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
		respondError(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	tool, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidTool):
			respondError(c, http.StatusBadRequest, "工具参数无效")

		case errors.Is(err, service.ErrServerNotFound):
			respondError(c, http.StatusNotFound, "所属 MCP Server 不存在")

		case errors.Is(err, service.ErrToolExists):
			respondError(c, http.StatusConflict, "同一 Server 下工具名称已存在")

		default:
			respondError(c, http.StatusInternalServerError, "工具注册失败")
		}
		return
	}
	respondSuccess(c, tool)
}
