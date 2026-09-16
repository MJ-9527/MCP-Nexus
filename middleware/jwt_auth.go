package middleware

import (
	"errors"
	"net/http"
	"strings"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth 认证中间件（B5）：校验 Bearer Token 的签名与过期时间，
// 通过后把用户身份注入请求上下文；缺失、伪造、过期统一返回 401。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		tokenStr, ok := strings.CutPrefix(raw, "Bearer ")
		if !ok || strings.TrimSpace(tokenStr) == "" {
			respondUnauthorized(c, "缺少认证令牌")
			return
		}
		claims := &service.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			// 算法固定 HS256，防止算法替换攻击
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			if errors.Is(err, jwt.ErrTokenExpired) {
				respondUnauthorized(c, "令牌已过期")
				return
			}
			respondUnauthorized(c, "无效令牌")
			return
		}
		// 注入身份供后续 RBAC / 审计使用（agent_role 与代理层 currentRole 的主键保持一致）
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("agent_role", claims.Role)
		c.Next()
	}
}

func respondUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":       "UNAUTHORIZED",
		"message":    message,
		"request_id": c.GetString("request_id"),
	})
}
