package handler

import (
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// SkillsImportHandler 处理 Skills 适配任务工具列表导入（B11）。
// 路由：POST /api/servers/:id/import-skills
type SkillsImportHandler struct {
	importer *service.SkillsImporter
}

func NewSkillsImportHandler(importer *service.SkillsImporter) *SkillsImportHandler {
	return &SkillsImportHandler{importer: importer}
}

// Import 解析请求体中的 Skills 工具列表，为每个工具在 mcp_tools 表创建/复用记录
// 并存储调用翻译元数据。导入后工具默认未发布，管理员审核后显式发布。
func (h *SkillsImportHandler) Import(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "server id 无效")
		return
	}
	var req model.ImportSkillsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "Skills 工具列表请求体格式错误")
		return
	}
	result, err := h.importer.Import(c.Request.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrServerNotFound):
			respondErrorCode(c, http.StatusNotFound, "SERVER_NOT_FOUND", "MCP Server 不存在")
		case errors.Is(err, service.ErrInvalidSkills):
			respondErrorCode(c, http.StatusBadRequest, "INVALID_SKILLS_SPEC", err.Error())
		default:
			respondErrorCode(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Skills 工具列表导入失败")
		}
		return
	}
	respondSuccess(c, result)
}
