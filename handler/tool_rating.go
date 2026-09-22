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

type ToolRatingHandler struct{ service *service.ToolRatingService }

func NewToolRatingHandler(s *service.ToolRatingService) *ToolRatingHandler {
	return &ToolRatingHandler{service: s}
}

type createRatingRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

func (h *ToolRatingHandler) Create(c *gin.Context) {
	toolID, userID, ok := ratingIDs(c)
	if !ok {
		return
	}
	var req createRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "评分参数无效")
		return
	}
	rating := &model.ToolRating{ToolID: toolID, UserID: userID, Rating: req.Rating, Comment: req.Comment}
	err := h.service.Create(c.Request.Context(), rating)
	if errors.Is(err, service.ErrInvalidRating) || errors.Is(err, service.ErrInvalidReview) {
		respondError(c, http.StatusBadRequest, "评分或评论内容无效")
		return
	}
	if errors.Is(err, repository.ErrConflict) {
		respondError(c, http.StatusConflict, "用户已经评价过该工具")
		return
	}
	if errors.Is(err, repository.ErrNotFound) || errors.Is(err, service.ErrToolNotFound) {
		respondError(c, http.StatusNotFound, "工具不存在")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "评分保存失败")
		return
	}
	c.JSON(http.StatusCreated, model.APIResponse{Code: "OK", Message: "created", RequestID: getRequestID(c), Data: rating})
}

func (h *ToolRatingHandler) List(c *gin.Context) {
	toolID, _, ok := ratingIDs(c)
	if !ok {
		return
	}
	page, size, err := parseRatingPage(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "分页参数无效")
		return
	}
	items, total, err := h.service.List(c.Request.Context(), toolID, page, size)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "评论查询失败")
		return
	}
	summary, err := h.service.Summary(c.Request.Context(), toolID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "评分汇总查询失败")
		return
	}
	respondSuccess(c, gin.H{"items": items, "total": total, "page": page, "page_size": size, "summary": summary})
}

func ratingIDs(c *gin.Context) (int64, int64, bool) {
	toolID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || toolID <= 0 {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return 0, 0, false
	}
	userID := c.GetInt64("user_id")
	if userID <= 0 {
		respondError(c, http.StatusUnauthorized, "缺少用户身份")
		return 0, 0, false
	}
	return toolID, userID, true
}

func parseRatingPage(c *gin.Context) (int, int, error) {
	page, size := 1, 20
	var err error
	if v := c.Query("page"); v != "" {
		page, err = strconv.Atoi(v)
		if err != nil || page <= 0 {
			return 0, 0, errors.New("page")
		}
	}
	if v := c.Query("page_size"); v != "" {
		size, err = strconv.Atoi(v)
		if err != nil || size <= 0 || size > 100 {
			return 0, 0, errors.New("page_size")
		}
	}
	return page, size, nil
}
