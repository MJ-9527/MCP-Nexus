package repository

import (
	"context"
	"fmt"
	"strings"

	"MCP-Nexus/model"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseAuditRepository struct {
	conn driver.Conn
}

func NewClickHouseAuditRepository(addr string) (*ClickHouseAuditRepository, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{Database: "default"},
	})
	if err != nil {
		return nil, fmt.Errorf("连接 ClickHouse 失败: %w", err)
	}
	return &ClickHouseAuditRepository{conn: conn}, nil
}

// EnsureSchema 建表（幂等），容器启动时调用。
func (r *ClickHouseAuditRepository) EnsureSchema(ctx context.Context) error {
	const ddl = `CREATE TABLE IF NOT EXISTS audit_logs (
		request_id String, caller_id Int64, role String, tool_name String,
		server_id Int64, args_summary String, called_at DateTime,
		latency_ms Int32, success UInt8, http_status Int32,
		reject_reason String, cost_estimate Float32
	) ENGINE = MergeTree() ORDER BY (called_at, tool_name)
	PARTITION BY toYYYYMM(called_at)`
	return r.conn.Exec(ctx, ddl)
}

func (r *ClickHouseAuditRepository) BatchInsert(ctx context.Context, logs []*model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	const stmt = `INSERT INTO audit_logs (request_id, caller_id, role, tool_name, server_id,
		args_summary, called_at, latency_ms, success, http_status, reject_reason, cost_estimate)`
	batch, err := r.conn.PrepareBatch(ctx, stmt)
	if err != nil {
		return err
	}
	for _, log := range logs {
		if err := batch.Append(
			log.RequestID, log.CallerID, log.Role, log.ToolName, log.ServerID,
			log.ArgsSummary, log.CalledAt, log.LatencyMS, log.Success,
			log.HTTPStatus, log.RejectReason, log.CostEstimate,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

// TopTools 近 since 小时内调用量最高的工具（市场排行）
func (r *ClickHouseAuditRepository) TopTools(ctx context.Context, sinceHours int, limit int) ([]model.RankingItem, error) {
	if sinceHours <= 0 {
		sinceHours = 24 * 7
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	// INTERVAL 语法不支持参数绑定，sinceHours 为 int 无注入风险，直接拼接
	query := fmt.Sprintf(`
		SELECT tool_name, COUNT(*) AS cnt
		FROM audit_logs
		WHERE called_at >= now() - INTERVAL %d HOUR
		GROUP BY tool_name
		ORDER BY cnt DESC
		LIMIT ?`, sinceHours)
	rows, err := r.conn.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]model.RankingItem, 0)
	for rows.Next() {
		var name string
		var cnt uint64 // ClickHouse COUNT(*) 返回 UInt64
		if err := rows.Scan(&name, &cnt); err != nil {
			return nil, err
		}
		items = append(items, model.RankingItem{ToolName: name, CallCount: int64(cnt)})
	}
	return items, rows.Err()
}

// ToolStats30d 工具近 30 天调用量与成功率（市场详情页）
func (r *ClickHouseAuditRepository) ToolStats30d(ctx context.Context, toolName string) (total int64, successRate float64, err error) {
	const query = `
		SELECT COUNT(*), AVG(success)
		FROM audit_logs
		WHERE tool_name = ? AND called_at >= now() - INTERVAL 720 HOUR`
	var cnt uint64 // ClickHouse COUNT(*) 返回 UInt64
	var avgSuccess float64
	if err := r.conn.QueryRow(ctx, query, toolName).Scan(&cnt, &avgSuccess); err != nil {
		return 0, 0, err
	}
	return int64(cnt), avgSuccess * 100, nil
}

// AuditLogFilter 审计日志查询过滤条件
type AuditLogFilter struct {
	ToolName   string
	Role       string
	Success    *bool // nil=全部，true=成功，false=失败
	SinceHours int   // 0=不限
	Page       int
	PageSize   int
}

// AuditLogItem 审计日志条目（查询返回）
type AuditLogItem = model.AuditLog

// QueryLogs 分页查询审计日志，支持按工具/角色/成功状态/时间范围过滤
func (r *ClickHouseAuditRepository) QueryLogs(ctx context.Context, f AuditLogFilter) ([]model.AuditLog, int64, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 20
	}
	conditions := []string{}
	args := []any{}
	if f.ToolName != "" {
		args = append(args, f.ToolName)
		conditions = append(conditions, "tool_name = ?")
	}
	if f.Role != "" {
		args = append(args, f.Role)
		conditions = append(conditions, "role = ?")
	}
	if f.Success != nil {
		var v uint8
		if *f.Success {
			v = 1
		}
		args = append(args, v)
		conditions = append(conditions, "success = ?")
	}
	if f.SinceHours > 0 {
		conditions = append(conditions, fmt.Sprintf("called_at >= now() - INTERVAL %d HOUR", f.SinceHours))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// COUNT
	var total uint64
	countQuery := "SELECT COUNT(*) FROM audit_logs" + where
	if err := r.conn.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 数据
	offset := (f.Page - 1) * f.PageSize
	args = append(args, f.PageSize, offset)
	dataQuery := "SELECT request_id, caller_id, role, tool_name, server_id, args_summary, called_at, latency_ms, success, http_status, reject_reason, cost_estimate FROM audit_logs" + where +
		" ORDER BY called_at DESC LIMIT ? OFFSET ?"
	rows, err := r.conn.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs := make([]model.AuditLog, 0)
	for rows.Next() {
		var log model.AuditLog
		var success uint8
		if err := rows.Scan(&log.RequestID, &log.CallerID, &log.Role, &log.ToolName, &log.ServerID, &log.ArgsSummary, &log.CalledAt, &log.LatencyMS, &success, &log.HTTPStatus, &log.RejectReason, &log.CostEstimate); err != nil {
			return nil, 0, err
		}
		log.Success = success == 1
		logs = append(logs, log)
	}
	return logs, int64(total), rows.Err()
}

// AnalyticsOverview 统计概览
type AnalyticsOverview struct {
	TotalCalls   int64               `json:"total_calls"`
	SuccessCount int64               `json:"success_count"`
	FailCount    int64               `json:"fail_count"`
	RejectCount  int64               `json:"reject_count"` // 权限拒绝
	SuccessRate  float64             `json:"success_rate"`
	AvgLatencyMS float64             `json:"avg_latency_ms"`
	Trend        []TrendPoint        `json:"trend"` // 近 7 天每日调用量
	TopTools     []model.RankingItem `json:"top_tools"`
	Alerts       []AlertItem         `json:"alerts"` // 敏感工具高频调用告警
}

// TrendPoint 每日趋势
type TrendPoint struct {
	Date        string  `json:"date"`
	Calls       int64   `json:"calls"`
	SuccessRate float64 `json:"success_rate"`
}

// AlertItem 告警条目
type AlertItem struct {
	ToolName string `json:"tool_name"`
	Calls    int64  `json:"calls"`
	Reason   string `json:"reason"`
}

// Overview 统计概览：总调用/成功率/拒绝/趋势/Top工具/告警
func (r *ClickHouseAuditRepository) Overview(ctx context.Context) (*AnalyticsOverview, error) {
	o := &AnalyticsOverview{
		Trend:    make([]TrendPoint, 0),
		TopTools: make([]model.RankingItem, 0),
		Alerts:   make([]AlertItem, 0),
	}

	// 总调用/成功/失败/拒绝/平均延迟
	const summaryQuery = `
		SELECT COUNT(*), sum(success), sum(if(reject_reason != '', 1, 0)), avg(success), avg(latency_ms)
		FROM audit_logs WHERE called_at >= now() - INTERVAL 168 HOUR`
	var total, successCnt uint64
	var rejectCnt uint64
	var avgSuccess, avgLatency float64
	if err := r.conn.QueryRow(ctx, summaryQuery).Scan(&total, &successCnt, &rejectCnt, &avgSuccess, &avgLatency); err != nil {
		return nil, err
	}
	o.TotalCalls = int64(total)
	o.SuccessCount = int64(successCnt)
	o.FailCount = o.TotalCalls - o.SuccessCount
	o.RejectCount = int64(rejectCnt)
	o.SuccessRate = avgSuccess * 100
	o.AvgLatencyMS = avgLatency

	// 近 7 天每日趋势
	const trendQuery = `
		SELECT formatDateTime(toDate(called_at), '%Y-%m-%d') AS d, COUNT(*), avg(success)
		FROM audit_logs WHERE called_at >= now() - INTERVAL 168 HOUR
		GROUP BY d ORDER BY d`
	rows, err := r.conn.Query(ctx, trendQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var dateStr string
		var calls uint64
		var sr float64
		if err := rows.Scan(&dateStr, &calls, &sr); err != nil {
			return nil, err
		}
		o.Trend = append(o.Trend, TrendPoint{Date: dateStr, Calls: int64(calls), SuccessRate: sr * 100})
	}

	// Top 工具
	if items, err := r.TopTools(ctx, 168, 10); err == nil {
		o.TopTools = items
	}

	// 敏感工具高频调用告警：近 1 小时内同工具调用 > 50 次
	const alertQuery = `
		SELECT tool_name, COUNT(*) AS cnt
		FROM audit_logs
		WHERE called_at >= now() - INTERVAL 1 HOUR
		GROUP BY tool_name
		HAVING cnt > 50
		ORDER BY cnt DESC`
	rows2, err := r.conn.Query(ctx, alertQuery)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var name string
		var cnt uint64
		if err := rows2.Scan(&name, &cnt); err != nil {
			return nil, err
		}
		o.Alerts = append(o.Alerts, AlertItem{
			ToolName: name, Calls: int64(cnt),
			Reason: "近 1 小时调用超过 50 次，疑似异常高频",
		})
	}

	return o, nil
}
