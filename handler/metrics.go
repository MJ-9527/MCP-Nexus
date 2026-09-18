package handler

import (
	"MCP-Nexus/client"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// MetricsHandler 指标查询入口（B14）。
// 路由：GET /api/metrics（admin 鉴权）
// 返回：调用耗时 P95/P99、成功率、按工具聚合 + 上游熔断器状态 + 审计 writer 状态。
type MetricsHandler struct {
	proxy *service.ProxyService
}

func NewMetricsHandler(proxy *service.ProxyService) *MetricsHandler {
	return &MetricsHandler{proxy: proxy}
}

// circuitStateName 将熔断状态枚举字符串化为可读名称。
func circuitStateName(state client.CircuitState) string {
	switch state {
	case client.CircuitClosed:
		return "closed"
	case client.CircuitOpen:
		return "open"
	case client.CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// GetMetrics GET /api/metrics 返回当前网关的可观测性快照（B14）：
// 调用耗时与成功率（按工具聚合）、上游熔断器状态、审计批量写入器状态。
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	snap := h.proxy.Metrics().Snapshot()
	// 上游熔断器状态（closed / open / half_open + 连续失败数）
	if breakers := h.proxy.Breakers(); breakers != nil {
		items := breakers.List()
		upstreams := make([]service.UpstreamStatus, 0, len(items))
		for _, it := range items {
			upstreams = append(upstreams, service.UpstreamStatus{
				Endpoint:     it.Endpoint,
				State:        circuitStateName(it.State),
				FailureCount: it.FailureCount,
			})
		}
		snap.Upstreams = upstreams
	}
	// 审计批量写入器状态（丢弃数 / flush 失败数 / 已写入数）
	if w := h.proxy.AuditWriter(); w != nil {
		snap.Writer = w.Stats()
	}
	respondSuccess(c, snap)
}
