package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// ProxyHandler 网关代理入口，处理工具发现与调用。
type ProxyHandler struct{ svc *service.ProxyService }

func NewProxyHandler(svc *service.ProxyService) *ProxyHandler {
	return &ProxyHandler{svc: svc}
}

// ListTools GET /mcp/tools —— 返回当前角色可见且可调用的工具。
// 角色来源：JWT 中间件注入的 gin context（key="role"）。
func (h *ProxyHandler) ListTools(c *gin.Context) {
	role := c.GetString("role")
	resp, err := h.svc.ListTools(c.Request.Context(), role)
	if err != nil {
		respondProxyError(c, err)
		return
	}
	respondSuccess(c, resp.Tools)
}

// CallTool POST /mcp/tools/:toolName/call —— 转发调用到下游 MCP Server。
// 请求体：{"arguments": {...}}
func (h *ProxyHandler) CallTool(c *gin.Context) {
	role := c.GetString("role")
	toolName := c.Param("toolName")
	if toolName == "" {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "toolName 缺失")
		return
	}
	var req model.McpToolCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	req.ToolName = toolName
	if req.Method == "" {
		req.Method = "tools/call"
	}
	callerID, _ := c.Get("user_id")
	uid, _ := callerID.(int64)
	res, err := h.svc.CallTool(c.Request.Context(), role, uid, &req)
	if err != nil {
		respondProxyError(c, err)
		return
	}
	respondSuccess(c, res)
}

// respondProxyError 将 ProxyService 错误映射为文档定义的业务错误码。
func respondProxyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		respondErrorCode(c, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, service.ErrPermissionUnavailable):
		respondErrorCode(c, http.StatusServiceUnavailable, "PERMISSION_UNAVAILABLE", err.Error())
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
	case errors.Is(err, service.ErrInvalidTool):
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
	default:
		respondError(c, http.StatusInternalServerError, "调用工具失败")
	}
}
