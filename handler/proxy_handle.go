package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"MCP-Nexus/model"
	"MCP-Nexus/service"
)

type ProxyHandler struct {
	proxyService *service.ProxyService
}

func NewProxyHandler(proxySvc *service.ProxyService) *ProxyHandler {
	return &ProxyHandler{proxyService: proxySvc}
}

// ListTools POST /gateway/tools/list
func (h *ProxyHandler) ListTools(c *gin.Context) {
	resp, err := h.proxyService.ListTools(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CallTool POST /gateway/tools/call
func (h *ProxyHandler) CallTool(c *gin.Context) {
	var req model.McpToolCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request body"})
		return
	}

	resp, err := h.proxyService.CallTool(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
