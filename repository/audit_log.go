package repository

import (
	"context"
	"time"

	"MCP-Nexus/model"
)

type AuditLogFilter struct {
	RequestID string
	UserID    *int64
	ToolID    *int64
	ServerID  *int64
	Status    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	// BatchCreate 批量写入审计日志（B14）。
	// 实现应一次性将所有记录提交到底层存储（如 PG multi-row INSERT），
	// 避免逐条往返造成 IO 抖动；由调用方保证切片非空且字段已脱敏。
	// 内存实现直接 append；自增 ID 由实现填充。
	BatchCreate(ctx context.Context, logs []*model.AuditLog) error
	List(ctx context.Context, filter AuditLogFilter) ([]*model.AuditLog, int64, error)
}
