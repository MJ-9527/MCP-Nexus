package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type RegisterHandler struct{ service *service.ServerService }

func NewRegisterHandler(s *service.ServerService) *RegisterHandler {
	return &RegisterHandler{service: s}
}

func (h *RegisterHandler) RegisterServer(c *gin.Context) {
	var req model.RegisterServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "请求参数无效")
		return
	}
	server, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidServer) {
			RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
			return
		}
		if errors.Is(err, service.ErrServerExists) {
			RespondError(c, http.StatusConflict, "SERVER_ALREADY_EXISTS", err.Error())
			return
		}
		RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器注册失败")
		return
	}
	RespondSuccess(c, server)
}
