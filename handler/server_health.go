package handler

import (
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type ServerHealthHandler struct{ service *service.ServerHealthService }

func NewServerHealthHandler(s *service.ServerHealthService) *ServerHealthHandler {
	return &ServerHealthHandler{service: s}
}

func (h *ServerHealthHandler) CheckServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "服务器 ID 无效")
		return
	}
	result, err := h.service.Check(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		RespondError(c, http.StatusNotFound, "SERVER_NOT_FOUND", "MCP Server 不存在")
		return
	}
	if errors.Is(err, service.ErrInvalidServer) {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "服务器 ID 无效")
		return
	}
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器健康检查失败")
		return
	}
	RespondSuccess(c, result)
}
