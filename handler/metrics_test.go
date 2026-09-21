package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

// fakeMetricsRepo 固定返回预设聚合结果，隔离 ClickHouse。
type fakeMetricsRepo struct {
	total      repository.ToolMetrics
	byTool     []repository.ToolMetrics
	err        error
	lastFilter repository.AuditMetricsFilter
}

func (f *fakeMetricsRepo) AggregateTotal(_ context.Context, filter repository.AuditMetricsFilter) (repository.ToolMetrics, error) {
	f.lastFilter = filter
	if f.err != nil {
		return repository.ToolMetrics{}, f.err
	}
	return f.total, nil
}

func (f *fakeMetricsRepo) AggregateByTool(_ context.Context, filter repository.AuditMetricsFilter) ([]repository.ToolMetrics, error) {
	f.lastFilter = filter
	if f.err != nil {
		return nil, f.err
	}
	return f.byTool, nil
}

// metricsResponse 解析 /api/metrics 的响应体。
type metricsResponse struct {
	Code string                  `json:"code"`
	Data service.MetricsSnapshot `json:"data"`
}

func callMetrics(t *testing.T, h *MetricsHandler, query string) (*httptest.ResponseRecorder, metricsResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics"+query, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.GetMetrics(c)
	var resp metricsResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w, resp
}

// B14：配置了 ClickHouse 指标源时，响应应由聚合结果生成，并标明来源与窗口。
func TestMetricsHandlerUsesClickHouseAggregates(t *testing.T) {
	proxy := service.NewProxyService(nil, nil, nil)
	repo := &fakeMetricsRepo{
		total: repository.ToolMetrics{ToolName: "__total__", Count: 10, SuccessCnt: 8, DeniedCnt: 1, FailedCnt: 1, P95MS: 120, P99MS: 200, AvgMS: 42.5, MaxMS: 260},
		byTool: []repository.ToolMetrics{
			{ToolName: "query_sales", Count: 6, SuccessCnt: 5, DeniedCnt: 1, P50MS: 30, P95MS: 90, P99MS: 120, AvgMS: 40, MaxMS: 120},
		},
	}
	h := NewMetricsHandler(proxy, repo)

	w, resp := callMetrics(t, h, "?window=1h")
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	if resp.Code != "OK" {
		t.Fatalf("code 应为 OK，实际 %s", resp.Code)
	}
	if resp.Data.Source != service.MetricsSourceClickHouse {
		t.Fatalf("来源应为 clickhouse，实际 %q", resp.Data.Source)
	}
	if resp.Data.Window != "1h0m0s" {
		t.Fatalf("窗口标签应为 1h0m0s，实际 %q", resp.Data.Window)
	}
	if resp.Data.Total.Count != 10 || resp.Data.Total.SuccessRate != 0.8 {
		t.Fatalf("总量指标不符: count=%d rate=%v", resp.Data.Total.Count, resp.Data.Total.SuccessRate)
	}
	if resp.Data.Total.P95MS != 120 || resp.Data.Total.P99MS != 200 {
		t.Fatalf("分位数不符: p95=%d p99=%d", resp.Data.Total.P95MS, resp.Data.Total.P99MS)
	}
	if len(resp.Data.ByTool) != 1 || resp.Data.ByTool[0].ToolName != "query_sales" {
		t.Fatalf("按工具聚合不符: %+v", resp.Data.ByTool)
	}
	if got := resp.Data.ByTool[0].SuccessRate; got < 0.83 || got > 0.84 {
		t.Fatalf("query_sales 成功率应约为 5/6，实际 %v", got)
	}
	// window=1h 应换算成 3600 秒传给聚合
	if repo.lastFilter.WindowSeconds != 3600 {
		t.Fatalf("聚合窗口应为 3600 秒，实际 %d", repo.lastFilter.WindowSeconds)
	}
}

// B14：分析存储不可用时降级为进程内指标，接口不应整体失败。
func TestMetricsHandlerFallsBackWhenClickHouseFails(t *testing.T) {
	proxy := service.NewProxyService(nil, nil, nil)
	h := NewMetricsHandler(proxy, &fakeMetricsRepo{err: errors.New("clickhouse down")})

	w, resp := callMetrics(t, h, "")
	if w.Code != http.StatusOK {
		t.Fatalf("降级后仍应返回 200，实际 %d", w.Code)
	}
	if resp.Data.Source != service.MetricsSourceMemory {
		t.Fatalf("降级来源应为 memory，实际 %q", resp.Data.Source)
	}
	if resp.Data.Window != "process" {
		t.Fatalf("降级窗口应为 process，实际 %q", resp.Data.Window)
	}
}

// 未配置分析存储时使用进程内指标。
func TestMetricsHandlerWithoutClickHouseUsesMemory(t *testing.T) {
	proxy := service.NewProxyService(nil, nil, nil)
	proxy.SetMetrics(service.NewMetricsCollector(64))
	proxy.Metrics().Observe("query_sales", 10, service.AuditStatusSuccess)

	h := NewMetricsHandler(proxy, nil)
	w, resp := callMetrics(t, h, "")
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	if resp.Data.Source != service.MetricsSourceMemory || resp.Data.Total.Count != 1 {
		t.Fatalf("应使用进程内指标: source=%q count=%d", resp.Data.Source, resp.Data.Total.Count)
	}
}

// B14：window 参数解析（默认 24h、天粒度、all、非法值、上限截断）。
func TestParseMetricsWindow(t *testing.T) {
	cases := []struct {
		raw       string
		wantLabel string
		wantSecs  int64
		wantErr   bool
	}{
		{"", "24h0m0s", 86400, false},
		{"30m", "30m0s", 1800, false},
		{"24h", "24h0m0s", 86400, false},
		{"7d", "168h0m0s", 604800, false},
		{"all", "all", 0, false},
		{"0", "all", 0, false},
		{"abc", "", 0, true},
		{"-5m", "", 0, true},
		{"0d", "", 0, true},
		// 超过 90 天上限应被截断
		{"200d", "2160h0m0s", 2160 * 3600, false},
	}
	for _, tc := range cases {
		label, secs, err := parseMetricsWindow(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("window=%q 应报错", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("window=%q 解析失败: %v", tc.raw, err)
			continue
		}
		if label != tc.wantLabel || secs != tc.wantSecs {
			t.Errorf("window=%q 期望 (%s, %d)，实际 (%s, %d)", tc.raw, tc.wantLabel, tc.wantSecs, label, secs)
		}
	}
}

// B14：非法 window 应返回 400，不触发聚合查询。
func TestMetricsHandlerRejectsInvalidWindow(t *testing.T) {
	proxy := service.NewProxyService(nil, nil, nil)
	repo := &fakeMetricsRepo{}
	h := NewMetricsHandler(proxy, repo)

	w, resp := callMetrics(t, h, "?window=oops")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("期望 400，实际 %d", w.Code)
	}
	if resp.Code != "INVALID_PARAMETER" {
		t.Fatalf("错误码应为 INVALID_PARAMETER，实际 %s", resp.Code)
	}
	if repo.lastFilter.WindowSeconds != 0 && repo.lastFilter != (repository.AuditMetricsFilter{}) {
		t.Fatalf("非法 window 不应触发聚合查询，实际 filter=%+v", repo.lastFilter)
	}
}
