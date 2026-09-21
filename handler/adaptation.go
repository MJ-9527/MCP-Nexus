package handler

import (
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"
	"errors"
	"github.com/gin-gonic/gin"
	"strconv"
)

type AdaptationHandler struct{ s *service.AdaptationService }

func NewAdaptationHandler(s *service.AdaptationService) *AdaptationHandler {
	return &AdaptationHandler{s: s}
}
func (h *AdaptationHandler) Create(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		respondError(c, 400, "工具 ID 无效")
		return
	}
	var req struct {
		TaskType  string `json:"task_type" binding:"required"`
		SourceURL string `json:"source_url"`
	}
	if c.ShouldBindJSON(&req) != nil {
		respondError(c, 400, "适配任务参数无效")
		return
	}
	t := &model.ToolAdaptationTask{ToolID: id, TaskType: req.TaskType, Status: "pending", SourceURL: req.SourceURL}
	if e = h.s.Create(c.Request.Context(), t); e != nil {
		if errors.Is(e, service.ErrInvalidAdaptation) {
			respondError(c, 400, "适配任务参数无效")
		} else if errors.Is(e, repository.ErrNotFound) {
			respondError(c, 404, "工具不存在")
		} else {
			respondError(c, 500, "创建适配任务失败")
		}
		return
	}
	c.JSON(201, model.APIResponse{Code: "OK", Message: "created", RequestID: getRequestID(c), Data: t})
}
func (h *AdaptationHandler) Get(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if e != nil || id <= 0 {
		respondError(c, 400, "任务 ID 无效")
		return
	}
	t, e := h.s.Get(c.Request.Context(), id)
	if errors.Is(e, repository.ErrNotFound) {
		respondError(c, 404, "任务不存在")
		return
	}
	if e != nil {
		respondError(c, 500, "任务查询失败")
		return
	}
	respondSuccess(c, t)
}
func (h *AdaptationHandler) List(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		respondError(c, 400, "工具 ID 无效")
		return
	}
	items, e := h.s.List(c.Request.Context(), id)
	if e != nil {
		respondError(c, 500, "任务查询失败")
		return
	}
	respondSuccess(c, gin.H{"items": items})
}

func (h *AdaptationHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil || id <= 0 {
		respondError(c, 400, "任务 ID 无效")
		return
	}
	var req struct {
		Status       string `json:"status" binding:"required"`
		ErrorMessage string `json:"error_message"`
	}
	if c.ShouldBindJSON(&req) != nil {
		respondError(c, 400, "任务状态参数无效")
		return
	}
	err = h.s.UpdateStatus(c.Request.Context(), id, req.Status, req.ErrorMessage)
	if errors.Is(err, service.ErrInvalidAdaptation) {
		respondError(c, 400, "任务状态无效")
		return
	}
	if errors.Is(err, repository.ErrNotFound) {
		respondError(c, 404, "任务不存在")
		return
	}
	if err != nil {
		respondError(c, 500, "任务状态更新失败")
		return
	}
	task, err := h.s.Get(c.Request.Context(), id)
	if err != nil {
		respondError(c, 500, "任务查询失败")
		return
	}
	respondSuccess(c, task)
}
