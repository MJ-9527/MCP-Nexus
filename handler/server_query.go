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
		respondError(c, http.StatusInternalServerError, "服务器查询失败")
		return
	}
	respondSuccess(c, gin.H{
		"items": servers,
		"total": len(servers),
	})
}

func (h *ServerQueryHandler) GetServer(c *gin.Context) {
	idText := c.Param("id")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "服务器ID无效")
		return
	}
	server, err := h.service.GetByID(
		c.Request.Context(),
		id,
	)
	if errors.Is(err, repository.ErrNotFound) {
		respondError(c, http.StatusNotFound, "MCP Server 不存在")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "查询服务器失败")
		return
	}
	respondSuccess(c, server)
}
