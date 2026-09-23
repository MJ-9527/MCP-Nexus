package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type HealthCheckHandler struct {
	service *service.HealthCheckService
}

func NewHealthCheckHandler(s *service.HealthCheckService) *HealthCheckHandler {
	return &HealthCheckHandler{service: s}
}

func (h *HealthCheckHandler) HealthCheck(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "服务器ID无效")
		return
	}

	result, err := h.service.HealthCheck(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrHealthServerNotFound) {
			respondError(c, http.StatusNotFound, "MCP Server 不存在")
			return
		}
		respondError(c, http.StatusInternalServerError, "健康检查失败")
		return
	}

	respondSuccess(c, gin.H{
		"server_id":     result.ServerID,
		"health_status": result.HealthStatus,
		"latency_ms":    result.LatencyMS,
		"checked_at":    result.CheckedAt.Format(time.RFC3339),
	})
}
