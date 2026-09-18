package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// MetricsHandler 指标查询入口（B14）。
// 路由：GET /api/metrics（admin 鉴权）
// 返回：调用耗时 P95/P99、成功率、按工具聚合 + 上游熔断器状态 + 审计 writer 状态。
//
// 数据来源优先级：配置了 ClickHouse 分析存储则按时间窗口从审计表聚合（进程重启不丢、
// 可回溯）；未配置或聚合失败时降级为进程内实时指标（响应里的 source/window 字段会标明）。
type MetricsHandler struct {
	proxy   *service.ProxyService
	metrics repository.AuditMetricsRepository // 可为 nil
}

func NewMetricsHandler(proxy *service.ProxyService, metrics repository.AuditMetricsRepository) *MetricsHandler {
	return &MetricsHandler{proxy: proxy, metrics: metrics}
}

const (
	// DefaultMetricsWindow 默认统计窗口。
	DefaultMetricsWindow = "24h"
	// MaxMetricsWindow 窗口上限，避免误传超大窗口拖垮分析存储查询。
	MaxMetricsWindow = 90 * 24 * time.Hour
)

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
// 可选查询参数 window：Go duration（30m / 24h）或 Nd（7d），all 表示不限时间，默认 24h。
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	windowLabel, windowSeconds, err := parseMetricsWindow(c.Query("window"))
	if err != nil {
		respondErrorCode(c, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	snap := h.snapshot(c.Request.Context(), windowLabel, windowSeconds)

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

// snapshot 优先从分析存储聚合；不可用时降级为进程内指标，保证接口不整体失败。
func (h *MetricsHandler) snapshot(ctx context.Context, windowLabel string, windowSeconds int64) service.MetricsSnapshot {
	if h.metrics == nil {
		return h.proxy.Metrics().Snapshot()
	}
	filter := repository.AuditMetricsFilter{WindowSeconds: windowSeconds}
	total, err := h.metrics.AggregateTotal(ctx, filter)
	if err != nil {
		log.Printf("[metrics] 分析存储总量聚合失败，降级为进程内指标：%v", err)
		return h.proxy.Metrics().Snapshot()
	}
	byTool, err := h.metrics.AggregateByTool(ctx, filter)
	if err != nil {
		log.Printf("[metrics] 分析存储按工具聚合失败，降级为进程内指标：%v", err)
		return h.proxy.Metrics().Snapshot()
	}
	return service.SnapshotFromAggregates(total, byTool, service.MetricsSourceClickHouse, windowLabel)
}

// parseMetricsWindow 解析 window 参数，返回窗口标签与秒数（<=0 表示不限时间）。
// 支持 Go duration（30m / 24h）与天粒度（7d），以及 all 表示全量。
func parseMetricsWindow(raw string) (string, int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = DefaultMetricsWindow
	}
	if raw == "all" || raw == "0" {
		return "all", 0, nil
	}

	var d time.Duration
	if strings.HasSuffix(raw, "d") {
		// Go 的 ParseDuration 不支持天，这里单独处理 Nd
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if err != nil || n <= 0 {
			return "", 0, invalidWindowError(raw)
		}
		d = time.Duration(n) * 24 * time.Hour
	} else {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			return "", 0, invalidWindowError(raw)
		}
		d = parsed
	}
	if d > MaxMetricsWindow {
		d = MaxMetricsWindow
	}
	return d.String(), int64(d.Seconds()), nil
}

func invalidWindowError(raw string) error {
	return fmt.Errorf("window 无效：%q（支持 30m / 24h / 7d 或 all，默认 %s）", raw, DefaultMetricsWindow)
}
