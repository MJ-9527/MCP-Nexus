package repository

import (
	"MCP-Nexus/model"
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type ClickHouseAuditAnalyticsSink struct{ conn clickhouse.Conn }

func NewClickHouseAuditAnalyticsSink(conn clickhouse.Conn) *ClickHouseAuditAnalyticsSink {
	return &ClickHouseAuditAnalyticsSink{conn: conn}
}
func (s *ClickHouseAuditAnalyticsSink) WriteBatch(ctx context.Context, logs []*model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO mcp_audit_logs (created_at,request_id,user_id,tool_id,server_id,tool_name,caller_role,status,http_status,duration_ms,denied_reason,reject_reason,params_digest,params_sensitive_masked,cost_estimate)`)
	if err != nil {
		return err
	}
	for _, v := range logs {
		if v == nil {
			continue
		}
		if err = batch.Append(v.CreatedAt, v.RequestID, v.UserID, v.ToolID, v.ServerID, v.ToolName, v.CallerRole, v.Status, uint16(v.HTTPStatus), uint64(v.DurationMS), v.DeniedReason, v.RejectReason, v.ParamsDigest, boolToUInt8(v.ParamsSensitiveMasked), v.CostEstimate); err != nil {
			return err
		}
	}
	return batch.Send()
}
func boolToUInt8(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}
