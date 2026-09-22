package handler

import (
	"MCP-Nexus/middleware"
	"MCP-Nexus/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RBAC enforces that the current user has permission to call the specified tool.
// Reads ?tool_id=<id> from query; skips check if absent or user is admin.
func RBAC(permSvc *service.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		toolIDStr := c.Query("tool_id")
		if toolIDStr == "" {
			c.Next()
			return
		}
		toolID, err := strconv.ParseInt(toolIDStr, 10, 64)
		if err != nil || toolID <= 0 {
			c.Next()
			return
		}
		userID, ok := middleware.GetCurrentUserID(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "未认证"})
			c.Abort()
			return
		}
		role := middleware.GetCurrentRole(c)
		if role == "admin" {
			c.Next()
			return
		}
		allowed, err := permSvc.HasPermission(c.Request.Context(), userID, toolID, "call")
		if err != nil || !allowed {
			c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "无权限调用该工具"})
			c.Abort()
			return
		}
		c.Next()
	}
}
