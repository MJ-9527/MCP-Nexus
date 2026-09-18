package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// B14：验证 GET /api/metrics 统计接口可返回采集到的调用指标。
func TestMetricsHandlerReturnsAggregatedMetrics(t *testing.T) {
	// 构造带指标采集器的 ProxyService（仓储依赖在指标路径上不使用，传 nil）
	proxy := service.NewProxyService(nil, nil, nil)
	metrics := service.NewMetricsCollector(64)
	proxy.SetMetrics(metrics)

	// 造 3 次调用：2 成功 + 1 失败
	metrics.Observe("query_sales", 10, service.AuditStatusSuccess)
	metrics.Observe("query_sales", 30, service.AuditStatusSuccess)
	metrics.Observe("query_sales", 20, service.AuditStatusFailed)

	h := NewMetricsHandler(proxy)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.GetMetrics(c)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	var resp struct {
		Code string                  `json:"code"`
		Data service.MetricsSnapshot `json:"data"`
	}
	body := w.Body.Bytes()
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("响应非合法 JSON: %v, body=%s", err, string(body))
	}
	if resp.Code != "OK" {
		t.Fatalf("code 应为 OK，实际 %s", resp.Code)
	}
	if resp.Data.Total.Count != 3 {
		t.Fatalf("总调用数应为 3，实际 %d", resp.Data.Total.Count)
	}
	if resp.Data.Total.SuccessCnt != 2 || resp.Data.Total.FailedCnt != 1 {
		t.Fatalf("成功/失败计数错误: success=%d failed=%d",
			resp.Data.Total.SuccessCnt, resp.Data.Total.FailedCnt)
	}
	if len(resp.Data.ByTool) != 1 || resp.Data.ByTool[0].ToolName != "query_sales" {
		t.Fatalf("应按工具聚合 query_sales，实际 %+v", resp.Data.ByTool)
	}
	if resp.Data.ByTool[0].P95MS == 0 {
		t.Fatalf("P95 应被计算，实际 %d", resp.Data.ByTool[0].P95MS)
	}
}

// B14：验证接口在未注入指标采集器时不 panic，返回可用兜底快照。
func TestMetricsHandlerWithoutCollector(t *testing.T) {
	proxy := service.NewProxyService(nil, nil, nil)
	h := NewMetricsHandler(proxy)

	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.GetMetrics(c)

	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
}
