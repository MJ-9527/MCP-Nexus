package handler

import (
	"MCP-Nexus/model"
	"net/http"

	"github.com/gin-gonic/gin"
)

func RespondSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, model.APIResponse{
		Code:      "OK",
		Message:   "ok",
		RequestID: getRequestID(c),
		Data:      data,
	})
}

func RespondError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, model.APIResponse{
		Code:      code,
		Message:   message,
		RequestID: getRequestID(c),
		Data:      nil,
	})
}

func Health(c *gin.Context) {
	RespondSuccess(c, gin.H{"status": "ok"})
}

func getRequestID(c *gin.Context) string {
	v, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
