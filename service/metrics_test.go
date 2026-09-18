package service

import (
	"testing"
)

// 验证：Observe 累计三类计数与样本量。
func TestMetricsObserveCounters(t *testing.T) {
	m := NewMetricsCollector(64)
	m.Observe("query_sales", 10, AuditStatusSuccess)
	m.Observe("query_sales", 20, AuditStatusSuccess)
	m.Observe("query_sales", 30, AuditStatusFailed)
	m.Observe("query_sales", 0, AuditStatusDenied)

	snap := m.Snapshot()
	if len(snap.ByTool) != 1 {
		t.Fatalf("应只聚合 1 个工具，实际 %d", len(snap.ByTool))
	}
	tool := snap.ByTool[0]
	if tool.ToolName != "query_sales" {
		t.Fatalf("工具名错误：%s", tool.ToolName)
	}
	if tool.Count != 4 {
		t.Fatalf("count 应为 4，实际 %d", tool.Count)
	}
	if tool.SuccessCnt != 2 || tool.DeniedCnt != 1 || tool.FailedCnt != 1 {
		t.Fatalf("分类计数错误 success=%d denied=%d failed=%d",
			tool.SuccessCnt, tool.DeniedCnt, tool.FailedCnt)
	}
	if tool.SuccessRate != 0.5 {
		t.Fatalf("成功率应为 0.5，实际 %f", tool.SuccessRate)
	}
	// 全局汇总应等同单一工具
	if snap.Total.Count != 4 {
		t.Fatalf("total count 应为 4，实际 %d", snap.Total.Count)
	}
}

// 验证：P50/P95/P99 计算采用 nearest-rank 方法。
func TestMetricsPercentileCalc(t *testing.T) {
	// 缓冲容量需 >= 样本数，否则环形覆盖会丢弃最旧样本（另见 TestMetricsRingBufferOverwrites）
	m := NewMetricsCollector(128)
	// 100 个样本：1..100（毫秒）
	for i := int64(1); i <= 100; i++ {
		m.Observe("tool", i, AuditStatusSuccess)
	}
	snap := m.Snapshot()
	if len(snap.ByTool) != 1 {
		t.Fatalf("应聚合 1 个工具，实际 %d", len(snap.ByTool))
	}
	tool := snap.ByTool[0]
	// nearest-rank：ceil(0.50*100)-1 = 49 → sorted[49] = 50
	if tool.P50MS != 50 {
		t.Fatalf("P50 应为 50，实际 %d", tool.P50MS)
	}
	// P95: ceil(0.95*100)-1 = 94 → sorted[94] = 95
	if tool.P95MS != 95 {
		t.Fatalf("P95 应为 95，实际 %d", tool.P95MS)
	}
	// P99: ceil(0.99*100)-1 = 98 → sorted[98] = 99
	if tool.P99MS != 99 {
		t.Fatalf("P99 应为 99，实际 %d", tool.P99MS)
	}
	if tool.MaxMS != 100 {
		t.Fatalf("Max 应为 100，实际 %d", tool.MaxMS)
	}
	if tool.AvgMS != 50.5 {
		t.Fatalf("Avg 应为 50.5，实际 %f", tool.AvgMS)
	}
}

// 验证：环形缓冲覆盖最旧样本（n > bufferSize 时只保留最近 N 条）。
func TestMetricsRingBufferOverwrites(t *testing.T) {
	m := NewMetricsCollector(8)
	// 写 16 个样本 1..16，环形缓冲只保留最后 8 个 (9..16)
	for i := int64(1); i <= 16; i++ {
		m.Observe("tool", i, AuditStatusSuccess)
	}
	snap := m.Snapshot()
	tool := snap.ByTool[0]
	if tool.Count != 16 {
		t.Fatalf("count 应累计 16，实际 %d", tool.Count)
	}
	// 样本数被裁剪到 bufferSize=8，Max 应为 16
	if tool.MaxMS != 16 {
		t.Fatalf("Max 应为 16（环形覆盖后保留 9..16），实际 %d", tool.MaxMS)
	}
	// P50 nearest-rank of 8 样本：ceil(0.5*8)-1=3 → sorted[3] = 12
	if tool.P50MS != 12 {
		t.Fatalf("P50 应为 12（基于样本 9..16），实际 %d", tool.P50MS)
	}
}

// 验证：多个工具分别聚合，互不串扰。
func TestMetricsMultiToolAggregation(t *testing.T) {
	m := NewMetricsCollector(64)
	m.Observe("alpha", 10, AuditStatusSuccess)
	m.Observe("alpha", 20, AuditStatusSuccess)
	m.Observe("beta", 100, AuditStatusFailed)

	snap := m.Snapshot()
	// 按调用数降序排列，alpha(2) 在 beta(1) 前
	if snap.ByTool[0].ToolName != "alpha" {
		t.Fatalf("alpha 应排第一，实际 %+v", snap.ByTool)
	}
	if snap.ByTool[0].Count != 2 || snap.ByTool[1].Count != 1 {
		t.Fatalf("各工具计数应独立：alpha=2 beta=1，实际 %+v", snap.ByTool)
	}
	// 全局汇总：3 条
	if snap.Total.Count != 3 {
		t.Fatalf("total count 应为 3，实际 %d", snap.Total.Count)
	}
}

// 验证：未观察任何调用时快照零值合理。
func TestMetricsSnapshotEmpty(t *testing.T) {
	m := NewMetricsCollector(64)
	snap := m.Snapshot()
	if snap.Total.Count != 0 || len(snap.ByTool) != 0 {
		t.Fatalf("空 collector 应返回零快照，实际 %+v", snap)
	}
}
