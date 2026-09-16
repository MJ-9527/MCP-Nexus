package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// ProxyHandler MCP 网关代理入口（B1）。
// 路由：GET  /mcp/tools                    工具发现
//
//	POST /mcp/tools/:toolName/call     工具调用转发
type ProxyHandler struct {
	proxyService *service.ProxyService
}

func NewProxyHandler(proxySvc *service.ProxyService) *ProxyHandler {
	return &ProxyHandler{proxyService: proxySvc}
}

// ListTools GET /mcp/tools 返回当前角色可调用的工具列表。
// 角色来源：JWT 中间件注入的 gin context（key=agent_role）；未接入 JWT 前由 X-Role 头提供（第 2 周 B5 接管）。
func (h *ProxyHandler) ListTools(c *gin.Context) {
	role := currentRole(c)
	resp, err := h.proxyService.ListTools(c.Request.Context(), role)
	if err != nil {
		respondProxyError(c, err)
		return
	}
	respondSuccess(c, resp)
}

// CallTool POST /mcp/tools/:toolName/call 请求体 {"arguments": {...}}，转发到下游 MCP Server。
func (h *ProxyHandler) CallTool(c *gin.Context) {
	toolName := c.Param("toolName")
	if toolName == "" {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "toolName 缺失")
		return
	}
	var req model.McpToolCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "请求体格式错误")
		return
	}
	req.ToolName = toolName
	if req.Method == "" {
		req.Method = "tools/call"
	}

	role := currentRole(c)
	requestID := getRequestID(c)
	resp, err := h.proxyService.CallTool(c.Request.Context(), role, &req, requestID)
	if err != nil {
		respondProxyError(c, err)
		return
	}
	respondSuccess(c, resp)
}

// currentRole 读取当前调用者角色。已通过 JWT 认证（user_id 存在）的请求只信任
// 令牌注入的角色，防止借 X-Role 头提权；未认证请求回退 X-Role / anonymous。
func currentRole(c *gin.Context) string {
	if _, authed := c.Get("user_id"); authed {
		if s, ok := c.Get("agent_role"); ok {
			if role, isStr := s.(string); isStr && role != "" {
				return role
			}
		}
		return "anonymous"
	}
	if r := c.GetHeader("X-Role"); r != "" {
		return r
	}
	return "anonymous"
}

// respondProxyError 将 service 层错误统一映射为业务错误码与 HTTP 状态码（B4）。
func respondProxyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		respondErrorCode(c, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, service.ErrToolNotFound):
		respondErrorCode(c, http.StatusNotFound, "TOOL_NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrToolOffline):
		respondErrorCode(c, http.StatusConflict, "TOOL_OFFLINE", err.Error())
	case errors.Is(err, service.ErrServerNotFound):
		respondErrorCode(c, http.StatusNotFound, "SERVER_NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrServerUnavailable):
		respondErrorCode(c, http.StatusServiceUnavailable, "SERVER_UNAVAILABLE", err.Error())
	case errors.Is(err, service.ErrUpstreamTimeout):
		respondErrorCode(c, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT", err.Error())
	case errors.Is(err, service.ErrUpstreamError):
		respondErrorCode(c, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
	default:
		respondError(c, http.StatusInternalServerError, "网关内部错误")
	}
}
