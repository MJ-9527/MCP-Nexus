package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ClickHouseMetricsRepository 从 ClickHouse 审计表聚合调用指标（B14）。
//
// 相比内存环形缓冲（service.MetricsCollector）的优势：
//   - 进程重启不丢历史
//   - 支持任意时间窗口（window 参数）
//   - 分位数由 ClickHouse 原生 quantile 计算，不受内存样本上限影响
type ClickHouseMetricsRepository struct {
	ch    *chHTTP
	table string
}

// NewClickHouseMetricsRepository 构造聚合查询器。table 为空取默认值。
func NewClickHouseMetricsRepository(addr, database, table, user, password string, timeout time.Duration) (*ClickHouseMetricsRepository, error) {
	ch, err := newCHHTTP(addr, database, user, password, timeout)
	if err != nil {
		return nil, err
	}
	if table == "" {
		table = DefaultClickHouseTable
	}
	if !chIdentifier.MatchString(table) {
		return nil, fmt.Errorf("invalid clickhouse table identifier %q", table)
	}
	return &ClickHouseMetricsRepository{ch: ch, table: table}, nil
}

// aggregateExpr 聚合表达式，顺序与 parseAggregateFields 的解析顺序严格一致。
// 字段数变化时两处必须同步修改。
const aggregateExpr = `count() AS cnt,
    countIf(status = 'success') AS ok,
    countIf(status = 'denied') AS denied,
    countIf(status = 'failed') AS failed,
    toInt64(round(quantile(0.5)(duration_ms))) AS p50,
    toInt64(round(quantile(0.95)(duration_ms))) AS p95,
    toInt64(round(quantile(0.99)(duration_ms))) AS p99,
    round(avg(duration_ms), 2) AS avg_ms,
    max(duration_ms) AS max_ms`

// aggregateFieldCount aggregateExpr 产生的列数，用于校验响应列数。
const aggregateFieldCount = 9

// tableRef 返回 db.table 限定名（库名与表名均已通过标识符校验）。
func (r *ClickHouseMetricsRepository) tableRef() string { return r.ch.qualified(r.table) }

// whereClause 生成时间窗口过滤；不限时间时返回空串。
func whereClause(filter AuditMetricsFilter) string {
	if filter.WindowSeconds > 0 {
		return fmt.Sprintf(" WHERE created_at >= now() - INTERVAL %d SECOND", filter.WindowSeconds)
	}
	return ""
}

// AggregateByTool 按工具聚合。空 tool_name（尚未回填的历史记录）归入 __unknown__。
func (r *ClickHouseMetricsRepository) AggregateByTool(ctx context.Context, filter AuditMetricsFilter) ([]ToolMetrics, error) {
	sql := fmt.Sprintf(
		"SELECT if(tool_name = '', '__unknown__', tool_name) AS tool, %s FROM %s%s GROUP BY tool ORDER BY cnt DESC FORMAT TabSeparated",
		aggregateExpr, r.tableRef(), whereClause(filter))
	body, err := r.ch.execute(ctx, sql)
	if err != nil {
		return nil, err
	}

	lines := splitNonEmptyLines(string(body))
	result := make([]ToolMetrics, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		if len(fields) != aggregateFieldCount+1 {
			return nil, fmt.Errorf("unexpected clickhouse metrics column count %d (want %d)", len(fields), aggregateFieldCount+1)
		}
		m, err := parseAggregateFields(fields[1:])
		if err != nil {
			return nil, err
		}
		m.ToolName = fields[0]
		result = append(result, m)
	}
	return result, nil
}

// AggregateTotal 全量聚合（不分组）。聚合查询恒返回一行。
func (r *ClickHouseMetricsRepository) AggregateTotal(ctx context.Context, filter AuditMetricsFilter) (ToolMetrics, error) {
	sql := fmt.Sprintf("SELECT %s FROM %s%s FORMAT TabSeparated", aggregateExpr, r.tableRef(), whereClause(filter))
	body, err := r.ch.execute(ctx, sql)
	if err != nil {
		return ToolMetrics{}, err
	}
	lines := splitNonEmptyLines(string(body))
	if len(lines) == 0 {
		// 无数据时 ClickHouse 仍返回一行（count=0），走到这里说明响应异常，按零值处理
		return ToolMetrics{ToolName: "__total__"}, nil
	}
	m, err := parseAggregateFields(strings.Split(lines[0], "\t"))
	if err != nil {
		return ToolMetrics{}, err
	}
	m.ToolName = "__total__"
	return m, nil
}

// parseAggregateFields 解析 aggregateExpr 对应的 9 个字段。
func parseAggregateFields(fields []string) (ToolMetrics, error) {
	if len(fields) != aggregateFieldCount {
		return ToolMetrics{}, fmt.Errorf("unexpected clickhouse aggregate column count %d (want %d)", len(fields), aggregateFieldCount)
	}
	var m ToolMetrics
	var err error
	if m.Count, err = strconv.ParseInt(fields[0], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse count: %w", err)
	}
	if m.SuccessCnt, err = strconv.ParseInt(fields[1], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse success count: %w", err)
	}
	if m.DeniedCnt, err = strconv.ParseInt(fields[2], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse denied count: %w", err)
	}
	if m.FailedCnt, err = strconv.ParseInt(fields[3], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse failed count: %w", err)
	}
	if m.P50MS, err = strconv.ParseInt(fields[4], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse p50: %w", err)
	}
	if m.P95MS, err = strconv.ParseInt(fields[5], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse p95: %w", err)
	}
	if m.P99MS, err = strconv.ParseInt(fields[6], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse p99: %w", err)
	}
	if m.AvgMS, err = strconv.ParseFloat(fields[7], 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse avg: %w", err)
	}
	if m.MaxMS, err = strconv.ParseInt(fields[8], 10, 64); err != nil {
		return ToolMetrics{}, fmt.Errorf("parse max: %w", err)
	}
	return m, nil
}

// splitNonEmptyLines 按行切分并去掉空行（TabSeparated 响应末尾带换行）。
func splitNonEmptyLines(body string) []string {
	raw := strings.Split(strings.TrimRight(body, "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// 编译期断言：满足指标聚合接口。
var _ AuditMetricsRepository = (*ClickHouseMetricsRepository)(nil)
