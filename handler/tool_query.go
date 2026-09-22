package handler

import (
	"MCP-Nexus/repository"
	"MCP-Nexus/service"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ToolQueryHandler struct{ service *service.ToolService }

func NewToolQueryHandler(s *service.ToolService) *ToolQueryHandler {
	return &ToolQueryHandler{service: s}
}

func (h *ToolQueryHandler) ListTools(c *gin.Context) {
	filter, err := parseToolFilter(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "工具查询参数无效")
		return
	}
	tools, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "工具查询失败")
		return
	}
	respondSuccess(c, gin.H{
		"items":     tools,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})
}

func (h *ToolQueryHandler) GetTool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return
	}
	tool, err := h.service.GetByID(c.Request.Context(), id)
	if errors.Is(err, service.ErrToolNotFound) {
		respondError(c, http.StatusNotFound, "工具不存在")
		return
	}
	if errors.Is(err, service.ErrInvalidTool) {
		respondError(c, http.StatusBadRequest, "工具 ID 无效")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "工具查询失败")
		return
	}
	respondSuccess(c, tool)
}

func parseToolFilter(c *gin.Context) (repository.ToolFilter, error) {
	filter := repository.ToolFilter{
		Name:         c.Query("q"),
		Keyword:      c.Query("keyword"),
		Category:     c.Query("category"),
		HealthStatus: c.Query("health_status"),
		Sort:         c.DefaultQuery("sort", "popularity"),
		Page:         1,
		PageSize:     20,
	}
	if filter.Keyword == "" {
		filter.Keyword = filter.Name
	}
	if value := c.Query("status"); value != "" {
		filter.Status = value
		if value == "published" {
			published := true
			filter.Published = &published
		}
	}
	if value := c.Query("tags"); value != "" {
		appendToolTags(&filter, value)
	}
	if value := c.Query("tag"); value != "" {
		appendToolTags(&filter, value)
	}
	if value, ok := c.GetQuery("published"); ok {
		published, err := strconv.ParseBool(value)
		if err != nil {
			return filter, err
		}
		filter.Published = &published
	}
	if value, ok := c.GetQuery("server_id"); ok {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			return filter, errors.New("invalid server_id")
		}
		filter.ServerID = &id
	}
	if value, ok := c.GetQuery("page"); ok {
		page, err := strconv.Atoi(value)
		if err != nil || page <= 0 {
			return filter, errors.New("invalid page")
		}
		filter.Page = page
	}
	if value, ok := c.GetQuery("page_size"); ok {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize <= 0 || pageSize > 100 {
			return filter, errors.New("invalid page_size")
		}
		filter.PageSize = pageSize
	}
	if !validToolSort(filter.Sort) {
		return filter, errors.New("invalid sort")
	}
	if filter.Status != "" && !validMarketStatus(filter.Status) {
		return filter, errors.New("invalid status")
	}
	if filter.HealthStatus != "" && !validToolHealthStatus(filter.HealthStatus) {
		return filter, errors.New("invalid health_status")
	}
	return filter, nil
}

func appendToolTags(filter *repository.ToolFilter, value string) {
	for _, tag := range strings.Split(value, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			filter.Tags = append(filter.Tags, tag)
		}
	}
}

func validToolSort(sort string) bool {
	switch sort {
	case "popularity", "rating", "name", "newest":
		return true
	default:
		return false
	}
}

func validMarketStatus(status string) bool {
	switch status {
	case "published", "all":
		return true
	default:
		return false
	}
}

func validToolHealthStatus(status string) bool {
	switch status {
	case "unknown", "online", "degraded", "offline":
		return true
	default:
		return false
	}
}
