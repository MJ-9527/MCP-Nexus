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
	respondErrorCode(c, status, httpCode(status), message)
}

// respondErrorCode 按业务错误码返回统一 JSON 结构（含 request_id），保证所有失败场景格式稳定。
func respondErrorCode(c *gin.Context, status int, code string, message string) {
	c.JSON(status, model.APIResponse{
		Code:      code,
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
	case 403:
		return "FORBIDDEN"
	case 401:
		return "UNAUTHORIZED"
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
