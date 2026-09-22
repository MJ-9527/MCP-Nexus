package service

import (
	"sort"
	"sync"
	"time"

	"MCP-Nexus/repository"
)

// B14 调用指标采集器。
//
// 设计目标：在内存中实时聚合每次工具调用的耗时与状态，供统计接口查询。
// 不依赖 ClickHouse：指标是「当前实时状态」的快照，无需历史回溯。
//
// 维度选择：按工具名（toolName）聚合。每个工具维护：
//   - 耗时环形缓冲（默认 1024 个样本，覆盖最近窗口）
//   - success/denied/failed 三类计数
//
// 计算 P95/P99 采用 nearest-rank 方法：对样本升序排序后取 ceil(p*n)-1 位置的值。
// 该方法对 1024 样本足够精确，且实现简单不需要 t-digest 等复杂结构。

const (
	// DefaultMetricsBufferSize 每工具保留的耗时样本数上限。
	// 取 1024 让 P99 至少基于 10 个样本（99% × 1024 ≈ 10），统计意义充分。
	DefaultMetricsBufferSize = 1024
)

// MetricsCollector 内存实时指标采集器（B14）。
type MetricsCollector struct {
	mu         sync.RWMutex
	bufferSize int
	perTool    map[string]*toolStats
	// 全局汇总（覆盖所有工具），便于一眼查看整体成功率与 P95/P99
	total *toolStats
}

// toolStats 单工具的统计状态。所有字段均在 MetricsCollector.mu 保护下访问。
type toolStats struct {
	samples    []int64 // 环形缓冲，写入位置 = count % bufferSize
	count      int64
	successCnt int64
	deniedCnt  int64
	failedCnt  int64
}

// ToolMetricSnapshot 单工具指标快照。
type ToolMetricSnapshot struct {
	ToolName    string  `json:"tool_name"`
	Count       int64   `json:"count"`
	SuccessCnt  int64   `json:"success_count"`
	DeniedCnt   int64   `json:"denied_count"`
	FailedCnt   int64   `json:"failed_count"`
	SuccessRate float64 `json:"success_rate"`
	P50MS       int64   `json:"p50_ms"`
	P95MS       int64   `json:"p95_ms"`
	P99MS       int64   `json:"p99_ms"`
	AvgMS       float64 `json:"avg_ms"`
	MaxMS       int64   `json:"max_ms"`
}

// UpstreamStatus 上游熔断器状态快照（B14 可观测性）。
type UpstreamStatus struct {
	Endpoint     string `json:"endpoint"`
	State        string `json:"state"` // closed / open / half_open
	FailureCount int    `json:"failure_count"`
}

// MetricsSnapshot 完整指标快照（响应 GET /api/metrics）。
type MetricsSnapshot struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Source      string               `json:"source"` // clickhouse=分析存储聚合；memory=进程内实时指标
	Window      string               `json:"window"` // 统计窗口，all 表示不限时间
	Total       ToolMetricSnapshot   `json:"total"`
	ByTool      []ToolMetricSnapshot `json:"by_tool"`
	Writer      WriterStats          `json:"writer"` // 审计批量写入器状态
	Upstreams   []UpstreamStatus     `json:"upstreams"`
}

// 指标来源标识。
const (
	MetricsSourceClickHouse = "clickhouse"
	MetricsSourceMemory     = "memory"
)

// SnapshotFromAggregates 把分析存储的聚合结果映射为 /api/metrics 响应结构（B14）。
// 字段与内存采集器输出保持一致，成功率由计数推导，前端无需区分来源。
// source 为 MetricsSourceClickHouse / MetricsSourceMemory；window 为窗口标签。
func SnapshotFromAggregates(total repository.ToolMetrics, byTool []repository.ToolMetrics, source, window string) MetricsSnapshot {
	snap := MetricsSnapshot{
		GeneratedAt: time.Now(),
		Source:      source,
		Window:      window,
		Total:       toToolMetricSnapshot("__total__", total),
		ByTool:      make([]ToolMetricSnapshot, 0, len(byTool)),
	}
	for _, item := range byTool {
		snap.ByTool = append(snap.ByTool, toToolMetricSnapshot(item.ToolName, item))
	}
	return snap
}

// toToolMetricSnapshot 聚合结果 → 响应结构，成功率 = 成功数 / 总数。
func toToolMetricSnapshot(name string, m repository.ToolMetrics) ToolMetricSnapshot {
	rate := 0.0
	if m.Count > 0 {
		rate = float64(m.SuccessCnt) / float64(m.Count)
	}
	return ToolMetricSnapshot{
		ToolName:    name,
		Count:       m.Count,
		SuccessCnt:  m.SuccessCnt,
		DeniedCnt:   m.DeniedCnt,
		FailedCnt:   m.FailedCnt,
		SuccessRate: rate,
		P50MS:       m.P50MS,
		P95MS:       m.P95MS,
		P99MS:       m.P99MS,
		AvgMS:       m.AvgMS,
		MaxMS:       m.MaxMS,
	}
}

// NewMetricsCollector 构造采集器。bufferSize <= 0 走默认值。
func NewMetricsCollector(bufferSize int) *MetricsCollector {
	if bufferSize <= 0 {
		bufferSize = DefaultMetricsBufferSize
	}
	return &MetricsCollector{
		bufferSize: bufferSize,
		perTool:    make(map[string]*toolStats),
		total:      newToolStats(bufferSize),
	}
}

func newToolStats(bufferSize int) *toolStats {
	return &toolStats{samples: make([]int64, bufferSize)}
}

// Observe 记录一次调用的耗时与状态。toolName 为空时计入 "__unknown__"。
// 调用链保证 Observe 永不失败，不影响主流程。
// 并发安全：单锁保护 perTool 与环形缓冲写入，避免同一工具并发写样本时的数据竞争。
func (m *MetricsCollector) Observe(toolName string, durationMS int64, status string) {
	if m == nil {
		return
	}
	if toolName == "" {
		toolName = "__unknown__"
	}
	if durationMS < 0 {
		durationMS = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ts, ok := m.perTool[toolName]
	if !ok {
		ts = newToolStats(m.bufferSize)
		m.perTool[toolName] = ts
	}
	recordSample(ts, durationMS, status)
	recordSample(m.total, durationMS, status)
}

// recordSample 写入环形缓冲与计数（非并发安全，调用方加锁）。
func recordSample(ts *toolStats, durationMS int64, status string) {
	pos := int(ts.count) % len(ts.samples)
	ts.samples[pos] = durationMS
	ts.count++
	switch status {
	case AuditStatusSuccess:
		ts.successCnt++
	case AuditStatusDenied:
		ts.deniedCnt++
	default:
		ts.failedCnt++
	}
}

// Snapshot 返回当前所有指标的快照。并发安全。
// upstreams 与 writer 由 handler 层从外部依赖（client 熔断器注册表、BatchAuditWriter）注入，
// 这里只负责聚合调用耗时指标。
func (m *MetricsCollector) Snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{GeneratedAt: time.Now(), Source: MetricsSourceMemory, Window: "process"}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	tools := make([]ToolMetricSnapshot, 0, len(m.perTool))
	for name, ts := range m.perTool {
		tools = append(tools, statsSnapshot(name, ts))
	}
	// 按调用数降序，便于观测热门工具
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Count > tools[j].Count
	})
	return MetricsSnapshot{
		GeneratedAt: time.Now(),
		Source:      MetricsSourceMemory,
		Window:      "process",
		Total:       statsSnapshot("__total__", m.total),
		ByTool:      tools,
	}
}

// statsSnapshot 聚合单个 toolStats 为可读快照（非并发安全，调用方加锁）。
func statsSnapshot(name string, ts *toolStats) ToolMetricSnapshot {
	count := ts.count
	snap := ToolMetricSnapshot{
		ToolName:   name,
		Count:      count,
		SuccessCnt: ts.successCnt,
		DeniedCnt:  ts.deniedCnt,
		FailedCnt:  ts.failedCnt,
	}
	if count > 0 {
		snap.SuccessRate = float64(ts.successCnt) / float64(count)
		// 取有效样本：环形缓冲写入位置之前的部分（写入未满一轮）或全部（已满）
		n := int(count)
		if n > len(ts.samples) {
			n = len(ts.samples)
		}
		sorted := make([]int64, n)
		copy(sorted, ts.samples[:n])
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		snap.P50MS = percentile(sorted, 0.50)
		snap.P95MS = percentile(sorted, 0.95)
		snap.P99MS = percentile(sorted, 0.99)
		var sum int64
		var max int64
		for i, v := range sorted {
			sum += v
			if i == 0 || v > max {
				max = v
			}
		}
		snap.AvgMS = float64(sum) / float64(n)
		snap.MaxMS = max
	}
	return snap
}

// percentile nearest-rank 方法：返回 ceil(p*n)-1 位置的值。
// n=0 返回 0；p 越界裁剪到 [0,1]。
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if p < 0 {
		p = 0
	} else if p > 1 {
		p = 1
	}
	// ceil(p*n) - 1，最小为 0
	idx := int(p*float64(len(sorted)) + 0.999999999)
	if idx <= 0 {
		idx = 1
	}
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}
