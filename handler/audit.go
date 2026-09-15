package handler

import (
	"net/http"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	perm *service.PermissionService
}

func NewAuditHandler(perm *service.PermissionService) *AuditHandler {
	return &AuditHandler{perm: perm}
}

// ListPermissions GET /api/permission/tools?role=xxx —— 返回某角色被允许调用的工具列表。
// 兼容现有 permission.Client 真实 HTTP 客户端的调用模式（内部 RBAC 实现）。
func (h *AuditHandler) ListPermissions(c *gin.Context) {
	role := c.Query("role")
	if role == "" {
		respondError(c, http.StatusBadRequest, "缺少 role 参数")
		return
	}
	tools, err := h.perm.GetAllowedToolNamesByRole(c.Request.Context(), role)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "查询权限失败")
		return
	}
	respondSuccess(c, gin.H{"role": role, "allowed_tools": tools})
}
