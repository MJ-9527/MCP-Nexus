package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 项目约定的角色名称（统一来源）：与 users.role 列、roles 表、限流配额、JWT claims 中的
// 角色字段保持一致。新增角色必须同步更新此常量集与 DefaultRoleLimits。
const (
	RoleAdmin     = "platform_admin" // 平台管理员：管理所有资源、配置权限、查看审计
	RoleDeveloper = "tool_developer" // 工具开发者：注册 Server、维护与发布自有工具
	RoleAgent     = "agent_caller"   // Agent 调用者：浏览工具、调用已授权工具
)

// RequireRole RBAC 中间件（B6）：校验 JWT 注入的主角色是否在允许名单内。
// 必须挂在 JWTAuth 之后；角色缺失或不匹配返回 403 并保留拒绝原因。
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(c *gin.Context) {
		role, _ := c.Get("agent_role")
		roleName, _ := role.(string)
		if roleName != "" && allowed[roleName] {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":          "FORBIDDEN",
			"message":       "权限不足：该操作需要角色 " + formatRoles(roles) + "，当前角色 " + roleName,
			"denied_reason": "role mismatch",
			"required_role": roles,
			"current_role":  roleName,
			"request_id":    c.GetString("request_id"),
		})
	}
}

func formatRoles(roles []string) string {
	out := ""
	for i, role := range roles {
		if i > 0 {
			out += " / "
		}
		out += role
	}
	return out
}
