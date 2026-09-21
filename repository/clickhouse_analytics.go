package repository

import (
	"MCP-Nexus/model"
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"time"
)

type ClickHouseAnalyticsRepository struct{ conn clickhouse.Conn }

func NewClickHouseAnalyticsRepository(c clickhouse.Conn) *ClickHouseAnalyticsRepository {
	return &ClickHouseAnalyticsRepository{conn: c}
}
func (r *ClickHouseAnalyticsRepository) Summary(ctx context.Context, start, end time.Time) (*model.AnalyticsSummary, error) {
	s := new(model.AnalyticsSummary)
	err := r.conn.QueryRow(ctx, `SELECT toInt64(count()),toInt64(countIf(status='success')),toInt64(countIf(status='failed')),toInt64(countIf(status IN ('denied','rejected'))),toFloat64(round(if(count()=0,0,countIf(status='success')*100.0/count()),2)),toFloat64(round(if(count()=0,0,countIf(status='failed')*100.0/count()),2)),toFloat64(if(count()=0,0,avg(duration_ms))),toFloat64(if(count()=0,0,quantile(0.99)(duration_ms))),toFloat64(sum(cost_estimate)) FROM mcp_audit_logs WHERE created_at >= ? AND created_at < ?`, start, end).Scan(&s.TotalCalls, &s.SuccessfulCalls, &s.FailedCalls, &s.RejectedCalls, &s.SuccessRate, &s.FailureRate, &s.AvgLatencyMS, &s.P99LatencyMS, &s.TotalCostEstimate)
	return s, err
}
func (r *ClickHouseAnalyticsRepository) Trends(ctx context.Context, start, end time.Time, g string, toolID *int64) ([]model.AnalyticsPoint, error) {
	bucket := "toStartOfHour(created_at)"
	if g == "day" {
		bucket = "toStartOfDay(created_at)"
	}
	q := `SELECT ` + bucket + `,toInt64(count()),toInt64(countIf(status='success')),toInt64(countIf(status='failed')),toInt64(countIf(status IN ('denied','rejected'))) FROM mcp_audit_logs WHERE created_at>=? AND created_at<?`
	args := []any{start, end}
	if toolID != nil {
		q += ` AND tool_id=?`
		args = append(args, *toolID)
	}
	q += ` GROUP BY 1 ORDER BY 1`
	rows, e := r.conn.Query(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.AnalyticsPoint{}
	for rows.Next() {
		var v model.AnalyticsPoint
		if e = rows.Scan(&v.Time, &v.Calls, &v.Success, &v.Failed, &v.Rejected); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ClickHouseAnalyticsRepository) TopTools(ctx context.Context, start, end time.Time, limit int) ([]model.ToolAnalyticsRank, error) {
	rows, e := r.conn.Query(ctx, `SELECT tool_id,tool_name,toInt64(count()),toFloat64(round(countIf(status='success')*100.0/count(),2)) rate FROM mcp_audit_logs WHERE created_at>=? AND created_at<? GROUP BY tool_id,tool_name ORDER BY count() DESC LIMIT ?`, start, end, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.ToolAnalyticsRank{}
	for rows.Next() {
		var v model.ToolAnalyticsRank
		if e = rows.Scan(&v.ToolID, &v.ToolName, &v.CallCount, &v.SuccessRate); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *ClickHouseAnalyticsRepository) RejectReasons(ctx context.Context, start, end time.Time, limit int) ([]model.RejectReasonStat, error) {
	rows, e := r.conn.Query(ctx, `SELECT if(reject_reason='',denied_reason,reject_reason) reason,toInt64(count()) FROM mcp_audit_logs WHERE created_at>=? AND created_at<? AND status IN ('denied','rejected') GROUP BY reason ORDER BY count() DESC LIMIT ?`, start, end, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.RejectReasonStat{}
	for rows.Next() {
		var v model.RejectReasonStat
		if e = rows.Scan(&v.Reason, &v.Count); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

var _ = fmt.Sprintf
