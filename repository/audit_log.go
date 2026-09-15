package repository

import (
	"context"

	"MCP-Nexus/model"
)

type AuditLogFilter struct {
	RequestID string
	UserID    *int64
	ToolID    *int64
	Status    string
	Limit     int
	Offset    int
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, filter AuditLogFilter) ([]*model.AuditLog, int64, error)
}
