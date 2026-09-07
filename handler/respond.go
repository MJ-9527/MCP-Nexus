package handler

import (
	"MCP-Nexus/model"

	"github.com/gin-gonic/gin"
)

func respondSuccess(c *gin.Context, data any) {
	c.JSON(200, model.APIResponse{
		Code:      "OK",
		Message:   "ok",
		RequestID: getRequestID(c),
		Data:      data,
	})
}

func respondError(c *gin.Context, status int, message string) {
	c.JSON(status, model.APIResponse{
		Code:      httpCode(status),
		Message:   message,
		RequestID: getRequestID(c),
		Data:      nil,
	})
}

func httpCode(status int) string {
	switch status {
	case 400:
		return "INVALID_PARAMETER"
	case 404:
		return "NOT_FOUND"
	case 409:
		return "CONFLICT"
	case 429:
		return "RATE_LIMITED"
	default:
		return "INTERNAL_ERROR"
	}
}

func getRequestID(c *gin.Context) string {
	requestID, exists := c.Get("request_id")
	if !exists {
		return ""
	}
	value, ok := requestID.(string)
	if !ok {
		return ""
	}
	return value
}
