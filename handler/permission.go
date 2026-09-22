package handler

import (
	"MCP-Nexus/middleware"
	"MCP-Nexus/model"
	"MCP-Nexus/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PermissionHandler struct{ svc *service.PermissionService }

func NewPermissionHandler(svc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{svc: svc}
}

func parseID(c *gin.Context, param string) (int64, error) {
	s := c.Param(param)
	if s == "" {
		s = c.Query(param)
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, err
	}
	return id, nil
}

func (h *PermissionHandler) Grant(c *gin.Context) {
	var req model.ToolPermission
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "权限参数无效")
		return
	}
	if req.ToolID <= 0 || req.Action == "" {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "tool_id 和 action 为必填")
		return
	}
	if req.UserID == nil && req.RoleID == nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "user_id 或 role_id 必须提供其中一个")
		return
	}
	if req.UserID != nil && req.RoleID != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "user_id 和 role_id 不能同时提供")
		return
	}
	if err := h.svc.Grant(c.Request.Context(), &req); err != nil {
		if errors.Is(err, service.ErrInvalidPermission) {
			RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
			return
		}
		if errors.Is(err, service.ErrPermissionExists) {
			RespondError(c, http.StatusConflict, "CONFLICT", "权限已存在")
			return
		}
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "授予权限失败")
		return
	}
	RespondSuccess(c, req)
}

func (h *PermissionHandler) Revoke(c *gin.Context) {
	toolID, err := parseID(c, "toolId")
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "toolId 无效")
		return
	}
	var req struct {
		UserID *int64 `json:"user_id"`
		RoleID *int64 `json:"role_id"`
		Action string `json:"action" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Action == "" {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "action 为必填")
		return
	}
	if req.UserID == nil && req.RoleID == nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "user_id 或 role_id 必须提供")
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), toolID, req.UserID, req.RoleID, req.Action); err != nil {
		if errors.Is(err, service.ErrPermissionNotFound) {
			RespondError(c, http.StatusNotFound, "NOT_FOUND", "权限记录不存在")
			return
		}
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "撤销权限失败")
		return
	}
	RespondSuccess(c, gin.H{"tool_id": toolID, "action": req.Action})
}

func (h *PermissionHandler) ListByTool(c *gin.Context) {
	toolID, err := parseID(c, "toolId")
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "toolId 无效")
		return
	}
	perms, err := h.svc.ListByTool(c.Request.Context(), toolID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "查询权限失败")
		return
	}
	RespondSuccess(c, gin.H{"items": perms, "total": len(perms)})
}

func (h *PermissionHandler) CheckPermission(c *gin.Context) {
	toolID, err := parseID(c, "toolId")
	if err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "toolId 无效")
		return
	}
	action := c.Query("action")
	if action == "" {
		action = "call"
	}
	userID, ok := middleware.GetCurrentUserID(c)
	if !ok {
		RespondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "未认证")
		return
	}
	has, err := h.svc.HasPermission(c.Request.Context(), userID, toolID, action)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "查询权限失败")
		return
	}
	RespondSuccess(c, gin.H{"tool_id": toolID, "user_id": userID, "action": action, "allowed": has})
}
