package repository

import (
	"MCP-Nexus/model"
	"context"
)

// AuditAnalyticsSink 是分析库写入边界。实现可以是 ClickHouse、文件队列或消息队列。
// WriteBatch 不应被主调用链同步调用；失败由上层记录并允许后续重试。
type AuditAnalyticsSink interface {
	WriteBatch(ctx context.Context, logs []*model.AuditLog) error
}
