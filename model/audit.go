package model

import "time"

// AuditLog 调用审计日志，字段对齐 ClickHouse audit_logs 表。
type AuditLog struct {
	RequestID    string    `json:"request_id" db:"request_id"`
	CallerID     int64     `json:"caller_id" db:"caller_id"`
	Role         string    `json:"role" db:"role"`
	ToolName     string    `json:"tool_name" db:"tool_name"`
	ServerID     int64     `json:"server_id" db:"server_id"`
	ArgsSummary  string    `json:"args_summary" db:"args_summary"`
	CalledAt     time.Time `json:"called_at" db:"called_at"`
	LatencyMS    int32     `json:"latency_ms" db:"latency_ms"`
	Success      bool      `json:"success" db:"success"`
	HTTPStatus   int32     `json:"http_status" db:"http_status"`
	RejectReason string    `json:"reject_reason" db:"reject_reason"`
	CostEstimate float32   `json:"cost_estimate" db:"cost_estimate"`
}
