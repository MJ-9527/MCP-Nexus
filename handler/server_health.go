package handler

import (
	"MCP-Nexus/repository"
	"MCP-Nexus/service"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type ServerHealthHandler struct{ service *service.ServerHealthService }

func NewServerHealthHandler(s *service.ServerHealthService) *ServerHealthHandler {
	return &ServerHealthHandler{service: s}
}
func (h *ServerHealthHandler) CheckServer(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "服务器 ID 无效")
		return
	}
	result, err := h.service.Check(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(c, http.StatusNotFound, "MCP Server 不存在")
		return
	}
	if errors.Is(err, service.ErrInvalidServer) {
		respondError(c, http.StatusBadRequest, "服务器 ID 无效")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "服务器健康检查失败")
		return
	}
	respondSuccess(c, result)
}
