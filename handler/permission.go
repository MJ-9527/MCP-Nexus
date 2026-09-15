package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// PermissionHandler 工具权限配置接口
type PermissionHandler struct{ svc *service.PermissionService }

func NewPermissionHandler(svc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{svc: svc}
}

// SetPermission POST /api/tools/:id/permissions
// 请求体: {"role":"agent","tool_name":"query_sales","action":"grant"}
// action 取值: grant（授权）/ revoke（撤销）
func (h *PermissionHandler) SetPermission(c *gin.Context) {
	var req model.SetPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	err := h.svc.SetPermission(c.Request.Context(), req.Role, req.ToolName, req.Action)
	if errors.Is(err, repository.ErrInvalidParameter) {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "角色、工具名称或操作类型无效")
		return
	}
	if err != nil {
		respondErrorCode(c, http.StatusInternalServerError, "PERMISSION_ERROR", err.Error())
		return
	}
	respondSuccess(c, gin.H{
		"role":      req.Role,
		"tool_name": req.ToolName,
		"action":    req.Action,
		"status":    "ok",
	})
}
