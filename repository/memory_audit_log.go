package repository

import (
	"context"
	"sync"

	"MCP-Nexus/model"
)

// MemoryAuditLogRepository 内存审计存储，用于单测与无依赖本地运行。
type MemoryAuditLogRepository struct {
	mu    sync.RWMutex
	logs  []*model.AuditLog
	scope int64
}

func NewMemoryAuditLogRepository() *MemoryAuditLogRepository {
	return &MemoryAuditLogRepository{}
}

func (r *MemoryAuditLogRepository) Create(_ context.Context, log *model.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scope++
	log.ID = r.scope
	r.logs = append(r.logs, log)
	return nil
}

func (r *MemoryAuditLogRepository) List(_ context.Context, filter AuditLogFilter) ([]*model.AuditLog, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*model.AuditLog, 0)
	for _, entry := range r.logs {
		if filter.RequestID != "" && entry.RequestID != filter.RequestID {
			continue
		}
		if filter.UserID != nil && (entry.UserID == nil || *entry.UserID != *filter.UserID) {
			continue
		}
		if filter.ToolID != nil && (entry.ToolID == nil || *entry.ToolID != *filter.ToolID) {
			continue
		}
		if filter.Status != "" && entry.Status != filter.Status {
			continue
		}
		result = append(result, entry)
	}
	total := int64(len(result))
	// 分页
	if filter.Offset > len(result) {
		result = nil
	} else {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}
	return result, total, nil
}

var _ AuditLogRepository = (*MemoryAuditLogRepository)(nil)
