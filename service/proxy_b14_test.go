package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// B14 E2E：多次调用后审计走批量写入，指标接口可查到聚合数据。
func TestCallToolMetricsAggregationAndBatchAudit(t *testing.T) {
	// 下游固定延迟 3ms：让调用耗时（毫秒级）非零，才能验证 P95 确实被采集
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Millisecond)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()

	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, downstream.URL)

	// 注入审计 writer 与 metrics
	auditRepo := repository.NewMemoryAuditLogRepository()
	auditSvc := NewAuditLogService(auditRepo)
	writer := NewBatchAuditWriter(auditRepo, 5, 30*time.Millisecond, 1024)
	auditSvc.SetBatchWriter(writer)
	writer.Start()
	defer writer.Stop()
	svc.SetAudit(auditSvc)
	metrics := NewMetricsCollector(64)
	svc.SetMetrics(metrics)

	// 5 次成功 + 1 次权限拒绝 + 1 次失败（工具不存在）
	for i := 0; i < 5; i++ {
		req := &model.McpToolCallRequest{ToolName: "query_sales", Arguments: map[string]any{"seq": i}}
		if _, err := svc.CallTool(context.Background(), "agent", 1, req, "req-b14-ok"); err != nil {
			t.Fatalf("调用 %d 失败: %v", i, err)
		}
	}
	// 拒绝：anonymous 角色
	_, errDenied := svc.CallTool(context.Background(), "anonymous", 999, &model.McpToolCallRequest{ToolName: "query_sales"}, "req-b14-denied")
	if !errors.Is(errDenied, ErrPermissionDenied) {
		t.Fatalf("期望权限拒绝，实际 %v", errDenied)
	}
	// 失败：工具不存在
	_, errFailed := svc.CallTool(context.Background(), "agent", 1, &model.McpToolCallRequest{ToolName: "ghost"}, "req-b14-fail")
	if !errors.Is(errFailed, ErrToolNotFound) {
		t.Fatalf("期望工具不存在，实际 %v", errFailed)
	}

	// 等待批量审计落地（短间隔会触发）
	logs, _, _ := auditRepo.List(context.Background(), repository.AuditLogFilter{})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		logs, _, _ = auditRepo.List(context.Background(), repository.AuditLogFilter{})
		if len(logs) >= 7 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(logs) < 7 {
		t.Fatalf("批量审计应落地 7 条，实际 %d", len(logs))
	}

	// 指标聚合
	snap := metrics.Snapshot()
	// query_sales: 5 success + 1 denied
	// __unknown__/ghost: 1 failed（callTool 在权限检查通过后失败，工具名是 ghost）
	if snap.Total.Count != 7 {
		t.Fatalf("总调用数应为 7，实际 %d", snap.Total.Count)
	}
	if snap.Total.SuccessCnt != 5 {
		t.Fatalf("成功计数应为 5，实际 %d", snap.Total.SuccessCnt)
	}
	if snap.Total.DeniedCnt != 1 {
		t.Fatalf("拒绝计数应为 1，实际 %d", snap.Total.DeniedCnt)
	}
	if snap.Total.FailedCnt != 1 {
		t.Fatalf("失败计数应为 1，实际 %d", snap.Total.FailedCnt)
	}
	if snap.Total.SuccessRate != 5.0/7.0 {
		t.Fatalf("成功率应为 5/7，实际 %f", snap.Total.SuccessRate)
	}
	// query_sales 单工具应有 P95 数据
	var qs *ToolMetricSnapshot
	for i := range snap.ByTool {
		if snap.ByTool[i].ToolName == "query_sales" {
			qs = &snap.ByTool[i]
			break
		}
	}
	if qs == nil {
		t.Fatalf("未找到 query_sales 指标")
	}
	if qs.Count != 6 { // 5 success + 1 denied
		t.Fatalf("query_sales count 应为 6，实际 %d", qs.Count)
	}
	// P95 取最大值样本（成功调用含 3ms 下游延迟）→ 应 >= 2ms
	if qs.P95MS < 2 {
		t.Fatalf("P95 应反映下游耗时（>=2ms），实际 %d", qs.P95MS)
	}
}

// B14：验证审计写入不阻塞调用链——大量并发调用下每条不应明显延迟。
func TestAuditBatchWriterNonBlocking(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer downstream.Close()
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, downstream.URL)

	auditRepo := repository.NewMemoryAuditLogRepository()
	auditSvc := NewAuditLogService(auditRepo)
	// 极小队列 + 极长间隔：只能靠 size=50 触发
	writer := NewBatchAuditWriter(auditRepo, 50, time.Hour, 16)
	auditSvc.SetBatchWriter(writer)
	writer.Start()
	defer writer.Stop()
	svc.SetAudit(auditSvc)

	// 并发 50 次调用，每次调用主流程不应阻塞
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.CallTool(context.Background(), "agent", 1, &model.McpToolCallRequest{ToolName: "query_sales"}, "req-nb")
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	// 50 次并发调用应在很短时间内完成（不含审计等待）；放宽阈值避免机器抖动
	if elapsed > 5*time.Second {
		t.Fatalf("并发 50 次调用耗时 %v，疑似审计阻塞调用链", elapsed)
	}
}
