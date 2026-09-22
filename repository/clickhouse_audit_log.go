package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"MCP-Nexus/model"
)

// ClickHouseAuditSink 通过 ClickHouse HTTP 接口批量写入审计日志（B14 分析存储）。
//
// 为什么走 HTTP 而不是原生协议：审计投递是「批量 + 尽力而为」的分析型写入，
// 每批最多 100 条（由 BatchAuditWriter 收敛），HTTP 往返开销可忽略；
// 同时免去额外驱动依赖，出问题时可直接用 curl 复现同样请求。
//
// 契约：只写不读，失败只向上返回 error，由调用方（BatchAuditWriter）记日志与计数，
// 不影响 PostgreSQL 主存储的审计记录。
type ClickHouseAuditSink struct {
	ch    *chHTTP
	table string
}

const (
	// DefaultClickHouseTable 审计分析表名。
	DefaultClickHouseTable = "mcp_audit_logs"
	// ClickHouseTimeLayout ClickHouse 的规范时间字面量格式（UTC、毫秒精度）。
	// 显式格式化为该形式，避免依赖 JSON 格式对 ISO8601 的兼容解析。
	ClickHouseTimeLayout = "2006-01-02 15:04:05.000"
)

// NewClickHouseAuditSink 构造分析存储写入器。addr 支持 "host:port" 或带 scheme。
// table 为空取默认值，非法标识符返回错误（由调用方决定是否降级为仅 PG）。
func NewClickHouseAuditSink(addr, database, table, user, password string, timeout time.Duration) (*ClickHouseAuditSink, error) {
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
	return &ClickHouseAuditSink{ch: ch, table: table}, nil
}

// auditLogRow 与 ClickHouse 表列名一一对应的 JSONEachRow 行。
// 字段顺序与名称必须与 db/clickhouse/init 下的建表语句保持一致。
type auditLogRow struct {
	RequestID             string  `json:"request_id"`
	UserID                *int64  `json:"user_id"`
	ToolID                *int64  `json:"tool_id"`
	ServerID              *int64  `json:"server_id"`
	ToolName              string  `json:"tool_name"`
	CallerRole            string  `json:"caller_role"`
	DurationMS            int64   `json:"duration_ms"`
	Status                string  `json:"status"`
	HTTPStatus            int     `json:"http_status"`
	DeniedReason          string  `json:"denied_reason"`
	RejectReason          string  `json:"reject_reason"`
	ParamsSummary         string  `json:"params_summary"`
	ParamsSensitiveMasked bool    `json:"params_sensitive_masked"`
	ParamsDigest          string  `json:"params_digest"`
	CostEstimate          float64 `json:"cost_estimate"`
	CreatedAt             string  `json:"created_at"`
}

// WriteBatch 批量写入审计记录。空批次直接返回；非 2xx 视为失败并带回上游错误片段。
func (s *ClickHouseAuditSink) WriteBatch(ctx context.Context, logs []*model.AuditLog) error {
	if s == nil || len(logs) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, log := range logs {
		if log == nil {
			continue
		}
		createdAt := log.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		encoded, err := json.Marshal(auditLogRow{
			RequestID:             log.RequestID,
			UserID:                log.UserID,
			ToolID:                log.ToolID,
			ServerID:              log.ServerID,
			ToolName:              log.ToolName,
			CallerRole:            log.CallerRole,
			DurationMS:            log.DurationMS,
			Status:                log.Status,
			HTTPStatus:            log.HTTPStatus,
			DeniedReason:          log.DeniedReason,
			RejectReason:          log.RejectReason,
			ParamsSummary:         log.ParamsSummary,
			ParamsSensitiveMasked: log.ParamsSensitiveMasked,
			ParamsDigest:          log.ParamsDigest,
			CostEstimate:          log.CostEstimate,
			CreatedAt:             createdAt.UTC().Format(ClickHouseTimeLayout),
		})
		if err != nil {
			return fmt.Errorf("marshal audit row: %w", err)
		}
		buf.Write(encoded)
		buf.WriteByte('\n')
	}
	if buf.Len() == 0 {
		return nil
	}
	return s.ch.writeBody(ctx,
		fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", s.ch.qualified(s.table)),
		"application/x-ndjson", buf.Bytes())
}

// 编译期断言：ClickHouse 写入器满足附加落地目标接口。
var _ AuditLogSink = (*ClickHouseAuditSink)(nil)
