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
	List(ctx context.Context, filter AuditLogFilter) ([]*model.AuditLog, int64, error)
}
