package handler

import (
	"errors"
	"net/http"

	"MCP-Nexus/model"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

type ServerRegisterHandler struct{ service *service.ServerService }

func NewRegisterHandler(s *service.ServerService) *ServerRegisterHandler {
	return &ServerRegisterHandler{service: s}
}

func (h *ServerRegisterHandler) RegisterServer(c *gin.Context) {
	var req model.RegisterServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数无效")
		return
	}
	server, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidServer) {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, service.ErrServerExists) {
			respondError(c, http.StatusConflict, err.Error())
			return
		}
		respondError(c, http.StatusInternalServerError, "服务器注册失败")
		return
	}
	respondSuccess(c, server)
}
