package handler

import (
	"MCP-Nexus/middleware"
	"MCP-Nexus/model"
	"MCP-Nexus/service"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ userService *service.UserService }

func NewAuthHandler(us *service.UserService) *AuthHandler {
	return &AuthHandler{userService: us}
}

// Login handles username+password login, returns a JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=2,max=64"`
		Password string `json:"password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "用户名或密码格式错误")
		return
	}
	user, err := h.userService.FindByUsername(c.Request.Context(), req.Username)
	if errors.Is(err, service.ErrUserNotFound) {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "用户名或密码错误")
		return
	}
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "登录失败，请稍后重试")
		return
	}
	// Demo: plain-text compare. In production use bcrypt.CompareHashAndPassword.
	if user.PasswordHash != req.Password {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "用户名或密码错误")
		return
	}
	token, err := middleware.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "生成 Token 失败")
		return
	}
	RespondSuccess(c, gin.H{
		"token":      token,
		"token_type": "Bearer",
		"expires_in": 86400,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

// Register creates a new user account.
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=2,max=64"`
		Password string `json:"password" binding:"required,min=6,max=128"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "注册信息无效")
		return
	}
	user := &model.User{
		Username:     req.Username,
		PasswordHash: req.Password, // demo: store plain; use bcrypt in production
		Status:       "active",
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if err := h.userService.Create(c.Request.Context(), user); err != nil {
		if errors.Is(err, service.ErrUserExists) {
			RespondError(c, http.StatusConflict, "CONFLICT", "用户名已存在")
			return
		}
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "注册失败")
		return
	}
	RespondSuccess(c, gin.H{"id": user.ID, "username": user.Username, "role": user.Role})
}
