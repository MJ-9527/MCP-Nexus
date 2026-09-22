package handler

import (
	"net/http"
	"strings"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// McpConfigHandler B9：为当前登录账号生成 MCP 接入配置。
type McpConfigHandler struct {
	configService *service.McpConfigService
	// publicBaseURL 来自配置 PUBLIC_BASE_URL；为空时用请求 Host 推导
	publicBaseURL string
}

func NewMcpConfigHandler(s *service.McpConfigService, publicBaseURL string) *McpConfigHandler {
	return &McpConfigHandler{configService: s, publicBaseURL: strings.TrimRight(publicBaseURL, "/")}
}

// GetMcpConfig GET /api/mcp-config
// 对外地址推导：优先环境变量 PUBLIC_BASE_URL；否则用请求 Host（反代场景信任 X-Forwarded-Proto）。
func (h *McpConfigHandler) GetMcpConfig(c *gin.Context) {
	role, userID := currentPrincipal(c)
	resp, err := h.configService.Generate(c.Request.Context(), role, userID, h.baseURL(c))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "生成接入配置失败")
		return
	}
	respondSuccess(c, resp)
}

func (h *McpConfigHandler) baseURL(c *gin.Context) string {
	if h.publicBaseURL != "" {
		return h.publicBaseURL
	}
	scheme := "http"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
