package repository

import "context"

// AuditMetricsFilter 调用指标聚合的过滤条件。
type AuditMetricsFilter struct {
	// WindowSeconds 只统计最近 N 秒的调用；<= 0 表示不限时间（全量）。
	// 用相对时间而非绝对时刻，避免网关与 ClickHouse 之间的时钟偏差。
	WindowSeconds int64
}

// ToolMetrics 按工具聚合的调用指标。
//
// 分位数口径说明：P50/P95/P99 由 ClickHouse 原生 quantile 计算，采用**线性插值**
// （如样本 [10,20,100] 的 P95 = 92）。进程内采集器 service.MetricsCollector 用的是
// nearest-rank（同例为 100）。两者都是合法分位定义，但数值会略有差异；
// /api/metrics 响应中的 source 字段标明当前数据来源，便于对账时区分。
// 选择原生 quantile 是因为它基于有损采样、内存开销可控；nearest-rank 需要把
// 分组内全部样本物化（groupArray）后再取位，在分析表规模下代价过高。
type ToolMetrics struct {
	ToolName   string
	Count      int64
	SuccessCnt int64
	DeniedCnt  int64
	FailedCnt  int64
	P50MS      int64
	P95MS      int64
	P99MS      int64
	AvgMS      float64
	MaxMS      int64
}

// AuditMetricsRepository 从分析存储（ClickHouse）聚合调用指标（B14）。
// 与内存采集器的区别：数据可回溯、进程重启不丢、支持时间窗口。
type AuditMetricsRepository interface {
	// AggregateByTool 按工具聚合，按调用量降序返回。
	AggregateByTool(ctx context.Context, filter AuditMetricsFilter) ([]ToolMetrics, error)
	// AggregateTotal 全量聚合（不分组）。
	AggregateTotal(ctx context.Context, filter AuditMetricsFilter) (ToolMetrics, error)
}
