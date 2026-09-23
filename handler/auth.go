package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type LoginRequest struct {
	Username string `json:"username" binding:"required,max=100"`
	Password string `json:"password" binding:"required,max=100"`
}

type AuthHandler struct{ service *service.AuthService }

func NewAuthHandler(s *service.AuthService) *AuthHandler {
	return &AuthHandler{service: s}
}

// Login 用户名密码登录，签发 JWT（B5）
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "用户名或密码格式无效")
		return
	}
	result, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		respondError(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if errors.Is(err, service.ErrUserDisabled) {
		respondError(c, http.StatusForbidden, "账号已被禁用")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "登录失败")
		return
	}
	respondSuccess(c, result)
}

// Me GET /api/auth/me 返回当前令牌对应的登录身份（对应 API 文档 2.3）。
// 身份信息以 JWT 注入的 user_id/username/agent_role 为准，不信任请求头。
func (h *AuthHandler) Me(c *gin.Context) {
	respondSuccess(c, gin.H{
		"id":       c.GetInt64("user_id"),
		"username": c.GetString("username"),
		"role":     c.GetString("agent_role"),
	})
}
