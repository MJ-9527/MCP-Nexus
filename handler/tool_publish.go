package handler

import (
	"MCP-Nexus/service"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
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
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return
	}
	err = h.service.SetPublished(c.Request.Context(), id, published)
	if errors.Is(err, service.ErrToolNotFound) {
		respondError(c, http.StatusNotFound, "工具不存在")
		return
	}
	if errors.Is(err, service.ErrInvalidTool) {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "更新工具发布状态失败")
		return
	}
	respondSuccess(c, gin.H{"id": id, "published": published})
}
