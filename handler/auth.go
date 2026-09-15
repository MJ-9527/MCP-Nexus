package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/auth/login —— 校验账号密码后签发 JWT。
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "用户名或密码不能为空")
		return
	}
	token, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		respondErrorCode(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}
	if errors.Is(err, service.ErrUserDisabled) {
		respondErrorCode(c, http.StatusForbidden, "USER_DISABLED", "用户已被禁用")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "登录失败")
		return
	}
	respondSuccess(c, token)
}
