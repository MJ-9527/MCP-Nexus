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

// ListTools GET /mcp/tools 返回当前调用者可查看/调用的工具列表。
// 身份来源：JWT 中间件（B5）注入的 user_id 与 agent_role（B6 RBAC 据此鉴权）。
func (h *ProxyHandler) ListTools(c *gin.Context) {
	role, userID := currentPrincipal(c)
	resp, err := h.proxyService.ListTools(c.Request.Context(), role, userID)
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

	role, userID := currentPrincipal(c)
	requestID := getRequestID(c)
	resp, err := h.proxyService.CallTool(c.Request.Context(), role, userID, &req, requestID)
	if err != nil {
		respondProxyError(c, err)
		return
	}
	respondSuccess(c, resp)
}

// currentPrincipal 读取当前调用者身份（角色 + 用户 ID）。已通过 JWT 认证（user_id 存在）的
// 请求只信任令牌注入的角色，防止借 X-Role 头提权；未认证请求回退 X-Role / anonymous。
func currentPrincipal(c *gin.Context) (string, int64) {
	userID := c.GetInt64("user_id")
	if userID > 0 {
		if s, ok := c.Get("agent_role"); ok {
			if role, isStr := s.(string); isStr && role != "" {
				return role, userID
			}
		}
		return "anonymous", userID
	}
	if r := c.GetHeader("X-Role"); r != "" {
		return r, 0
	}
	return "anonymous", 0
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
	case errors.Is(err, service.ErrInvalidArguments):
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETERS", err.Error())
	case errors.Is(err, service.ErrVersionMismatch):
		respondErrorCode(c, http.StatusBadRequest, "VERSION_MISMATCH", err.Error())
	case errors.Is(err, service.ErrUpstreamTimeout):
		respondErrorCode(c, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT", err.Error())
	case errors.Is(err, service.ErrUpstreamError):
		respondErrorCode(c, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
	default:
		respondError(c, http.StatusInternalServerError, "网关内部错误")
	}
}
