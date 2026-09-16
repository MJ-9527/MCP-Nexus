package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type ToolPermissionHandler struct {
	service *service.ToolPermissionService
}

func NewToolPermissionHandler(s *service.ToolPermissionService) *ToolPermissionHandler {
	return &ToolPermissionHandler{service: s}
}

func (h *ToolPermissionHandler) GrantPermission(c *gin.Context) {
	toolID, ok := parseID(c)
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		respondError(c, http.StatusBadRequest, "权限参数无效")
		return
	}
	payload, _ := json.Marshal(raw)
	if _, ok := raw["permissions"]; ok {
		var req model.ConfigureToolPermissionsRequest
		if err := json.Unmarshal(payload, &req); err != nil || len(req.Permissions) == 0 {
			respondError(c, http.StatusBadRequest, "权限矩阵参数无效")
			return
		}
		if _, err := h.service.Configure(c.Request.Context(), toolID, req); err != nil {
			h.respondPermissionError(c, err)
			return
		}
		respondSuccess(c, gin.H{"tool_id": toolID, "updated": true})
		return
	}
	var req model.GrantToolPermissionRequest
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&req); err != nil {
		respondError(c, http.StatusBadRequest, "权限参数无效")
		return
	}
	permission, err := h.service.Grant(c.Request.Context(), toolID, req)
	if err != nil {
		h.respondPermissionError(c, err)
		return
	}
	respondSuccess(c, permission)
}

func (h *ToolPermissionHandler) RevokePermission(c *gin.Context) {
	toolID, ok := parseID(c)
	if !ok {
		return
	}
	var req model.GrantToolPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "权限参数无效")
		return
	}
	if err := h.service.Revoke(c.Request.Context(), toolID, req); err != nil {
		h.respondPermissionError(c, err)
		return
	}
	respondSuccess(c, gin.H{"tool_id": toolID, "revoked": true})
}

func (h *ToolPermissionHandler) ListPermissions(c *gin.Context) {
	toolID, ok := parseID(c)
	if !ok {
		return
	}
	permissions, err := h.service.ListByTool(c.Request.Context(), toolID)
	if err != nil {
		h.respondPermissionError(c, err)
		return
	}
	tool, err := h.service.Tool(ctxOrRequest(c), toolID)
	if err != nil {
		h.respondPermissionError(c, err)
		return
	}
	grouped := make(map[int64]gin.H)
	for _, permission := range permissions {
		if permission.RoleID == nil {
			continue
		}
		role, err := h.service.Role(ctxOrRequest(c), *permission.RoleID)
		if err != nil {
			continue
		}
		item, ok := grouped[*permission.RoleID]
		if !ok {
			item = gin.H{"role": role.Name, "can_read": false, "can_call": false}
			grouped[*permission.RoleID] = item
		}
		if permission.Action == "read" {
			item["can_read"] = true
		}
		if permission.Action == "call" {
			item["can_call"] = true
		}
	}
	items := make([]gin.H, 0, len(grouped))
	for _, item := range grouped {
		items = append(items, item)
	}
	respondSuccess(c, gin.H{"tool_id": toolID, "is_sensitive": tool.IsSensitive, "sensitive_level": tool.SensitiveLevel, "permissions": items})
}

func ctxOrRequest(c *gin.Context) context.Context { return c.Request.Context() }

func (h *ToolPermissionHandler) CheckPermission(c *gin.Context) {
	toolID, ok := parseID(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		respondError(c, http.StatusBadRequest, "用户 ID 无效")
		return
	}
	action := strings.TrimSpace(c.Query("action"))
	allowed, err := h.service.Check(c.Request.Context(), userID, toolID, action)
	if err != nil {
		h.respondPermissionError(c, err)
		return
	}
	respondSuccess(c, gin.H{"allowed": allowed, "user_id": userID, "tool_id": toolID, "action": action})
}

func (h *ToolPermissionHandler) respondPermissionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPermission):
		respondError(c, http.StatusBadRequest, "权限参数无效")
	case errors.Is(err, service.ErrPermissionDenied):
		respondError(c, http.StatusForbidden, "没有工具调用权限")
	case errors.Is(err, service.ErrPermissionNotFound):
		respondError(c, http.StatusNotFound, "工具、主体或权限不存在")
	case errors.Is(err, service.ErrPermissionExists):
		respondError(c, http.StatusConflict, "权限已存在")
	default:
		respondError(c, http.StatusInternalServerError, "权限操作失败")
	}
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return 0, false
	}
	return id, true
}
