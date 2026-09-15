package middleware

import (
	"MCP-Nexus/model"
	"MCP-Nexus/service"
	"strings"

	"github.com/gin-gonic/gin"
)

// Auth JWT 校验中间件。从 Authorization: Bearer <token> 解析 JWT，
// 将 user_id / role 注入 gin context。失败返回 401。
func Auth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(401, model.APIResponse{
				Code:      "UNAUTHORIZED",
				Message:   "缺少有效的 Authorization 头",
				RequestID: c.GetString("request_id"),
			})
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(401, model.APIResponse{
				Code:      "INVALID_TOKEN",
				Message:   "JWT 校验失败",
				RequestID: c.GetString("request_id"),
			})
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireRole 限定角色访问（须在 Auth 之后使用）
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetRole(c)
		for _, r := range roles {
			if r == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, model.APIResponse{
			Code:      "FORBIDDEN",
			Message:   "需要以下角色之一: " + strings.Join(roles, ","),
			RequestID: c.GetString("request_id"),
		})
	}
}

// GetRole 从 gin context 取 role（Auth 中间件注入）。
func GetRole(c *gin.Context) string {
	if v, ok := c.Get("role"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "anonymous"
}

// GetUserID 从 gin context 取 user_id。
func GetUserID(c *gin.Context) int64 {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}
