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

// AuditLogSink 审计日志的附加落地目标（B14）。
// 与 AuditLogRepository 的区别：只写不读，用于把同一批审计记录额外投递到
// 分析型存储（ClickHouse）。
//
// 约定：签名中的 Addr 为空表示未配置该目标；投递失败只记日志与计数，
// 不影响主存储写入，也不阻塞调用链（由 BatchAuditWriter 负责调度与容错）。
type AuditLogSink interface {
	WriteBatch(ctx context.Context, logs []*model.AuditLog) error
}
