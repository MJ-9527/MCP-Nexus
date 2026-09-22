package handler

import (
	"MCP-Nexus/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AlertsHandler struct{ svc *service.AlertService }

func NewAlertsHandler(svc *service.AlertService) *AlertsHandler {
	return &AlertsHandler{svc: svc}
}

func (h *AlertsHandler) List(c *gin.Context) {
	handledStr := c.Query("handled")
	var handled *bool
	if handledStr != "" {
		v, err := strconv.ParseBool(handledStr)
		if err == nil {
			handled = &v
		}
	}
	alerts := h.svc.List(handled)
	RespondSuccess(c, gin.H{"items": alerts, "total": len(alerts)})
}

func (h *AlertsHandler) Acknowledge(c *gin.Context) {
	alertID := c.Param("alertId")
	if alertID == "" {
		RespondError(c, http.StatusBadRequest, "INVALID_PARAMETER", "alertId 为必填")
		return
	}
	ok := h.svc.Acknowledge(alertID)
	if !ok {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "告警不存在")
		return
	}
	RespondSuccess(c, gin.H{"alert_id": alertID, "handled": true})
}