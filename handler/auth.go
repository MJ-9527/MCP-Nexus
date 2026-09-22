package handler

import (
<<<<<<< HEAD
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
=======
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
>>>>>>> origin/pull-request
}
