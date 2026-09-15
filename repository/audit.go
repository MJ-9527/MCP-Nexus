package repository

import (
	"MCP-Nexus/model"
	"context"
)

// AuditRepository 审计日志写入接口。
type AuditRepository interface {
	// BatchInsert 批量写入审计日志。
	BatchInsert(ctx context.Context, logs []*model.AuditLog) error
}
