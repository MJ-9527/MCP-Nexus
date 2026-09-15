package handler

import (
	"errors"
	"net/http"
	"strconv"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// MarketHandler 工具市场接口
type MarketHandler struct{ svc *service.MarketService }

func NewMarketHandler(svc *service.MarketService) *MarketHandler {
	return &MarketHandler{svc: svc}
}

// ListTools GET /api/market/tools?category=&keyword=&page=&page_size=
func (h *MarketHandler) ListTools(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.svc.List(c.Request.Context(),
		c.Query("category"), c.Query("keyword"), page, pageSize)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "查询市场工具失败")
		return
	}
	respondSuccess(c, resp)
}

// GetTool GET /api/market/tools/:id
func (h *MarketHandler) GetTool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具 ID 非法")
		return
	}
	resp, err := h.svc.Detail(c.Request.Context(), id)
	if err != nil {
		respondMarketError(c, err)
		return
	}
	respondSuccess(c, resp)
}

// UpsertReview POST /api/market/tools/:id/reviews —— 登录用户提交/更新评分评论
func (h *MarketHandler) UpsertReview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具 ID 非法")
		return
	}
	userID, ok := c.Get("user_id")
	if !ok {
		respondErrorCode(c, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return
	}
	uid, _ := userID.(int64)
	var req model.UpsertReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "评分必须为 1-5 的整数")
		return
	}
	review, err := h.svc.UpsertReview(c.Request.Context(), id, uid, &req)
	if err != nil {
		respondMarketError(c, err)
		return
	}
	respondSuccess(c, review)
}

// ListReviews GET /api/market/tools/:id/reviews
func (h *MarketHandler) ListReviews(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具 ID 非法")
		return
	}
	reviews, err := h.svc.ListReviews(c.Request.Context(), id)
	if err != nil {
		respondMarketError(c, err)
		return
	}
	respondSuccess(c, reviews)
}

// Ranking GET /api/market/ranking?limit=10
func (h *MarketHandler) Ranking(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	items, err := h.svc.Ranking(c.Request.Context(), limit)
	if err != nil {
		respondErrorCode(c, http.StatusInternalServerError, "RANKING_QUERY_ERROR", err.Error())
		return
	}
	respondSuccess(c, items)
}

// ConfigSnippet GET /api/market/tools/:id/config-snippet
func (h *MarketHandler) ConfigSnippet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "工具 ID 非法")
		return
	}
	resp, err := h.svc.ConfigSnippet(c.Request.Context(), id, c.Query("gateway_url"))
	if err != nil {
		respondMarketError(c, err)
		return
	}
	respondSuccess(c, resp)
}

func respondMarketError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		respondErrorCode(c, http.StatusNotFound, "TOOL_NOT_FOUND", "工具不存在")
	case errors.Is(err, service.ErrToolOffline):
		respondErrorCode(c, http.StatusConflict, "TOOL_OFFLINE", "工具已下线")
	default:
		respondError(c, http.StatusInternalServerError, "市场操作失败")
	}
}

// IntegrationHandler OpenAPI / Skills 导入接口
type IntegrationHandler struct {
	openapi *service.OpenAPIService
	skills  *service.SkillsService
}

func NewIntegrationHandler(openapi *service.OpenAPIService, skills *service.SkillsService) *IntegrationHandler {
	return &IntegrationHandler{openapi: openapi, skills: skills}
}

// ImportOpenAPI POST /api/openapi/import
func (h *IntegrationHandler) ImportOpenAPI(c *gin.Context) {
	var req model.OpenAPIImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "请求体无效（需要 spec 字段）")
		return
	}
	resp, err := h.openapi.Import(c.Request.Context(), &req)
	if err != nil {
		var us *service.ErrUnsupportedDetail
		if errors.As(err, &us) {
			respondErrorCode(c, http.StatusBadRequest, "UNSUPPORTED_OPENAPI_FEATURE", us.Detail)
			return
		}
		respondErrorCode(c, http.StatusBadRequest, "OPENAPI_PARSE_ERROR", err.Error())
		return
	}
	respondSuccess(c, resp)
}

// ImportSkills POST /api/skills/import
func (h *IntegrationHandler) ImportSkills(c *gin.Context) {
	var req model.SkillImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", "请求体无效（需要 skills 数组）")
		return
	}
	resp, err := h.skills.Import(c.Request.Context(), &req)
	if err != nil {
		respondErrorCode(c, http.StatusBadRequest, "SKILL_IMPORT_ERROR", err.Error())
		return
	}
	respondSuccess(c, resp)
}
