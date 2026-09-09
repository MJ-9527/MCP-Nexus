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
	// 从gin上下文获取JWT中间件注入的agent_role
	roleVal, exists := c.Get("agent_role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized, missing role"})
		return
	}
	role := roleVal.(string)

	resp, err := h.proxyService.ListTools(c.Request.Context(), role)
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

	// 获取角色
	roleVal, exists := c.Get("agent_role")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized, missing role"})
		return
	}
	role := roleVal.(string)

	resp, err := h.proxyService.CallTool(c.Request.Context(), role, &req)
	if err != nil {
		// 权限不足单独返回403
		if err.Error() == "permission denied: agent cannot call this tool" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
