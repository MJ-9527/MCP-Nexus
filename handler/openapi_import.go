package handler

import (
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// OpenAPIImportHandler 处理 OpenAPI/Swagger 规范导入（B10）。
// 路由：POST /api/servers/:id/import-openapi
type OpenAPIImportHandler struct {
	importer *service.OpenAPIImporter
}

func NewOpenAPIImportHandler(importer *service.OpenAPIImporter) *OpenAPIImportHandler {
	return &OpenAPIImportHandler{importer: importer}
}

// Import 解析请求体中的 OpenAPI/Swagger 规范，为每个 path+method 创建/复用 MCP 工具
// 并存储翻译元数据。导入后工具默认未发布，管理员审核后显式发布。
func (h *OpenAPIImportHandler) Import(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "server id 无效")
		return
	}
	var req model.ImportOpenAPIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "OpenAPI 规范请求体格式错误")
		return
	}
	result, err := h.importer.Import(c.Request.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrServerNotFound):
			respondErrorCode(c, http.StatusNotFound, "SERVER_NOT_FOUND", "MCP Server 不存在")
		case errors.Is(err, service.ErrInvalidOpenAPI):
			respondErrorCode(c, http.StatusBadRequest, "INVALID_OPENAPI_SPEC", err.Error())
		default:
			respondErrorCode(c, http.StatusInternalServerError, "INTERNAL_ERROR", "OpenAPI 规范导入失败")
		}
		return
	}
	respondSuccess(c, result)
}
