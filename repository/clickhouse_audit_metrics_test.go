package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeCHQuery 假 ClickHouse 查询端点：记录收到的 SQL，按队列依次返回预设响应。
type fakeCHQuery struct {
	mu        sync.Mutex
	queries   []string
	responses []string
	status    int
	index     int
}

func (f *fakeCHQuery) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	_ = raw
	f.mu.Lock()
	f.queries = append(f.queries, r.URL.Query().Get("query"))
	idx := f.index
	if idx >= len(f.responses) {
		idx = len(f.responses) - 1
	}
	body := ""
	if len(f.responses) > 0 {
		body = f.responses[idx]
	}
	f.index++
	status := f.status
	f.mu.Unlock()

	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (f *fakeCHQuery) captured() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.queries...)
}

// B14：按工具聚合应解析 TabSeparated 响应，并把窗口换算成 INTERVAL 条件。
func TestClickHouseMetricsAggregateByTool(t *testing.T) {
	fake := &fakeCHQuery{responses: []string{
		"query_sales\t6\t5\t1\t0\t30\t90\t120\t40.5\t120\n" +
			"__unknown__\t4\t3\t1\t0\t10\t50\t60\t20\t60\n",
	}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	repo, err := NewClickHouseMetricsRepository(srv.URL, "mcp_analytics", "audit_logs", "", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.AggregateByTool(context.Background(), AuditMetricsFilter{WindowSeconds: 3600})
	if err != nil {
		t.Fatalf("聚合失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("应返回 2 个工具，实际 %d", len(got))
	}
	first := got[0]
	if first.ToolName != "query_sales" || first.Count != 6 || first.SuccessCnt != 5 || first.DeniedCnt != 1 || first.FailedCnt != 0 {
		t.Fatalf("计数解析错误: %+v", first)
	}
	if first.P50MS != 30 || first.P95MS != 90 || first.P99MS != 120 || first.MaxMS != 120 {
		t.Fatalf("分位数解析错误: %+v", first)
	}
	if first.AvgMS != 40.5 {
		t.Fatalf("均值解析错误: %v", first.AvgMS)
	}
	if got[1].ToolName != "__unknown__" {
		t.Fatalf("空工具名应由 ClickHouse 归入 __unknown__，实际 %q", got[1].ToolName)
	}

	sql := fake.captured()[0]
	for _, want := range []string{
		"FROM mcp_analytics.audit_logs",
		"GROUP BY tool",
		"WHERE created_at >= now() - INTERVAL 3600 SECOND",
		"quantile(0.95)(duration_ms)",
		"FORMAT TabSeparated",
		"if(tool_name = '', '__unknown__', tool_name)",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("SQL 缺少 %q\n实际: %s", want, sql)
		}
	}
}

// B14：不限时间时不应带 WHERE 条件；总量聚合无 GROUP BY。
func TestClickHouseMetricsAggregateTotalNoWindow(t *testing.T) {
	fake := &fakeCHQuery{responses: []string{"10\t8\t1\t1\t50\t120\t200\t42.5\t260\n"}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	repo, err := NewClickHouseMetricsRepository(srv.URL, "mcp_analytics", "audit_logs", "", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.AggregateTotal(context.Background(), AuditMetricsFilter{})
	if err != nil {
		t.Fatalf("聚合失败: %v", err)
	}
	if got.Count != 10 || got.SuccessCnt != 8 || got.DeniedCnt != 1 || got.FailedCnt != 1 {
		t.Fatalf("总量计数解析错误: %+v", got)
	}
	if got.P95MS != 120 || got.P99MS != 200 || got.AvgMS != 42.5 || got.MaxMS != 260 {
		t.Fatalf("总量分位数解析错误: %+v", got)
	}
	if got.ToolName != "__total__" {
		t.Fatalf("总量标签应为 __total__，实际 %q", got.ToolName)
	}

	sql := fake.captured()[0]
	if strings.Contains(sql, "WHERE") {
		t.Errorf("不限时间不应带 WHERE 条件: %s", sql)
	}
	if strings.Contains(sql, "GROUP BY") {
		t.Errorf("总量聚合不应有 GROUP BY: %s", sql)
	}
}

// B14：列数异常应报错，避免静默返回错位数据。
func TestClickHouseMetricsRejectsUnexpectedColumnCount(t *testing.T) {
	fake := &fakeCHQuery{responses: []string{"1\t2\t3\n"}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	repo, err := NewClickHouseMetricsRepository(srv.URL, "mcp_analytics", "audit_logs", "", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AggregateTotal(context.Background(), AuditMetricsFilter{}); err == nil {
		t.Fatal("列数不足应返回错误")
	}

	fake2 := &fakeCHQuery{responses: []string{"query_sales\t1\t2\t3\n"}}
	srv2 := httptest.NewServer(fake2)
	defer srv2.Close()
	repo2, err := NewClickHouseMetricsRepository(srv2.URL, "mcp_analytics", "audit_logs", "", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo2.AggregateByTool(context.Background(), AuditMetricsFilter{}); err == nil {
		t.Fatal("按工具聚合列数不足应返回错误")
	}
}

// B14：ClickHouse 返回非 2xx 时应把上游错误片段带出来。
func TestClickHouseMetricsSurfacesUpstreamError(t *testing.T) {
	fake := &fakeCHQuery{status: http.StatusInternalServerError, responses: []string{"Code: 60. DB::Exception: Table doesn't exist"}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	repo, err := NewClickHouseMetricsRepository(srv.URL, "mcp_analytics", "audit_logs", "", "", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.AggregateTotal(context.Background(), AuditMetricsFilter{})
	if err == nil || !strings.Contains(err.Error(), "doesn't exist") {
		t.Fatalf("应带出上游错误片段，实际 %v", err)
	}
}

// B14：非法表名应被拒绝。
func TestNewClickHouseMetricsRepositoryRejectsInvalidTable(t *testing.T) {
	if _, err := NewClickHouseMetricsRepository("localhost:8123", "mcp_analytics", "audit logs", "", "", time.Second); err == nil {
		t.Fatal("非法表名应被拒绝")
	}
}
