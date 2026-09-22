package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"MCP-Nexus/model"
)

// 对真实 ClickHouse 的聚合查询集成验证：确认 SQL 语法、分位数函数、
// TabSeparated 解析与时间窗口过滤在真实实例上都成立。
//
// 默认跳过；设置 CLICKHOUSE_HTTP_ADDR 后启用：
//
//	CLICKHOUSE_HTTP_ADDR=127.0.0.1:8123 go test ./repository/ -run ClickHouseMetricsIntegration -v
//
// 前置条件：目标库已应用 db/clickhouse/init/001_create_audit_logs.sql。
func TestClickHouseMetricsIntegrationAggregates(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("CLICKHOUSE_HTTP_ADDR"))
	if addr == "" {
		t.Skip("未设置 CLICKHOUSE_HTTP_ADDR，跳过 ClickHouse 集成测试")
	}
	db := os.Getenv("CLICKHOUSE_DB")
	if db == "" {
		db = "mcp_analytics"
	}

	sink, err := NewClickHouseAuditSink(addr, db, "audit_logs", "", "", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewClickHouseMetricsRepository(addr, db, "audit_logs", "", "", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// 用唯一工具名隔离本次数据，避免受表中既有记录影响
	toolName := fmt.Sprintf("it_metrics_%d", time.Now().UnixNano())
	now := time.Now().UTC()
	rows := []*model.AuditLog{
		{RequestID: toolName + "-1", ToolName: toolName, CallerRole: "agent", DurationMS: 10, Status: "success", CreatedAt: now},
		{RequestID: toolName + "-2", ToolName: toolName, CallerRole: "agent", DurationMS: 20, Status: "success", CreatedAt: now},
		{RequestID: toolName + "-3", ToolName: toolName, CallerRole: "agent", DurationMS: 100, Status: "denied", CreatedAt: now},
	}
	if err := sink.WriteBatch(context.Background(), rows); err != nil {
		t.Fatalf("写入测试数据失败: %v", err)
	}
	// ClickHouse 写入可见，但聚合前给一点缓冲，避免偶发可见性延迟
	time.Sleep(300 * time.Millisecond)

	filter := AuditMetricsFilter{WindowSeconds: 3600}
	byTool, err := repo.AggregateByTool(context.Background(), filter)
	if err != nil {
		t.Fatalf("按工具聚合失败: %v", err)
	}
	var found *ToolMetrics
	for i := range byTool {
		if byTool[i].ToolName == toolName {
			found = &byTool[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("聚合结果中未找到 %s：%+v", toolName, byTool)
	}
	if found.Count != 3 || found.SuccessCnt != 2 || found.DeniedCnt != 1 || found.FailedCnt != 0 {
		t.Fatalf("计数不符: %+v", found)
	}
	// 样本 [10, 20, 100]：ClickHouse 原生 quantile 为线性插值（P95=92），
	// 与内存采集器的 nearest-rank（100）口径不同，故这里只约束区间与最大值。
	if found.P95MS < 20 || found.P95MS > 100 {
		t.Fatalf("P95 应落在 [20,100] 区间，实际 %d", found.P95MS)
	}
	if found.MaxMS != 100 {
		t.Fatalf("Max 应为 100，实际 %d", found.MaxMS)
	}
	if found.P50MS < 10 || found.P50MS > 100 {
		t.Fatalf("P50 应落在 [10,100] 区间，实际 %d", found.P50MS)
	}

	// 总量聚合（不分组）应覆盖本次记录
	total, err := repo.AggregateTotal(context.Background(), filter)
	if err != nil {
		t.Fatalf("总量聚合失败: %v", err)
	}
	if total.Count < 3 {
		t.Fatalf("总量应至少包含本次 3 条，实际 %d", total.Count)
	}
	if total.ToolName != "__total__" {
		t.Fatalf("总量标签应为 __total__，实际 %q", total.ToolName)
	}

	// 清理本次测试数据
	chQuery(t, addr, db, fmt.Sprintf(
		"ALTER TABLE %s.audit_logs DELETE WHERE tool_name = '%s'", db, toolName))
}
