package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type ServerStatusHandler struct{ service *service.ServerService }

func NewServerStatusHandler(s *service.ServerService) *ServerStatusHandler {
	return &ServerStatusHandler{service: s}
}

func (h *ServerStatusHandler) ActivateServer(c *gin.Context) {
	h.setStatus(c, h.service.Activate, "active")
}
func (h *ServerStatusHandler) OfflineServer(c *gin.Context) {
	h.setStatus(c, h.service.Offline, "offline")
}

func (h *ServerStatusHandler) setStatus(c *gin.Context, update func(context.Context, int64) error, status string) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "服务器 ID 无效")
		return
	}
	if err := update(c.Request.Context(), id); errors.Is(err, repository.ErrNotFound) {
		respondError(c, http.StatusNotFound, "MCP Server 不存在")
		return
	} else if errors.Is(err, service.ErrInvalidServerStatusTransition) {
		respondError(c, http.StatusConflict, "不允许的服务器状态转换")
		return
	} else if err != nil {
		respondError(c, http.StatusInternalServerError, "更新服务器状态失败")
		return
	}
	respondSuccess(c, gin.H{"id": id, "status": status})
}
