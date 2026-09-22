package handler

import (
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type ServerQueryHandler struct {
	service *service.ServerService
}

func NewServerQueryHandler(s *service.ServerService) *ServerQueryHandler {
	return &ServerQueryHandler{service: s}
}

func (h *ServerQueryHandler) ListServers(c *gin.Context) {
	servers, err := h.service.List(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器查询失败")
		return
	}
	RespondSuccess(c, gin.H{
		"items": servers,
		"total": len(servers),
	})
}

func (h *ServerQueryHandler) GetServer(c *gin.Context) {
	idText := c.Param("id")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "服务器ID无效")
		return
	}
	server, err := h.service.GetByID(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		RespondError(c, http.StatusNotFound, "SERVER_NOT_FOUND", "MCP Server 不存在")
		return
	}
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "查询服务器失败")
		return
	}
	RespondSuccess(c, server)
}
