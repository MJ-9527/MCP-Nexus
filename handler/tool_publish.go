package handler

import (
	"MCP-Nexus/service"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ToolPublishHandler struct{ service *service.ToolService }

func NewToolPublishHandler(s *service.ToolService) *ToolPublishHandler {
	return &ToolPublishHandler{service: s}
}
func (h *ToolPublishHandler) PublishTool(c *gin.Context) { h.setPublished(c, true) }
func (h *ToolPublishHandler) OfflineTool(c *gin.Context) { h.setPublished(c, false) }
func (h *ToolPublishHandler) setPublished(c *gin.Context, published bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具 ID 无效")
		return
	}
	err = h.service.SetPublished(c.Request.Context(), id, published)
	if errors.Is(err, service.ErrToolNotFound) {
		RespondError(c, http.StatusNotFound, "TOOL_NOT_FOUND", "工具不存在")
		return
	}
	if errors.Is(err, service.ErrInvalidTool) {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具 ID 无效")
		return
	}
	if err != nil {
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "更新工具发布状态失败")
		return
	}
	RespondSuccess(c, gin.H{"id": id, "published": published})
}
