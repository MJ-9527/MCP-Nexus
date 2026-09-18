package repository

import (
	"context"
	"sync"
	"time"

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
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	r.logs = append(r.logs, log)
	return nil
}

// BatchCreate 批量 append，自增 ID 顺序填充（B14）。
func (r *MemoryAuditLogRepository) BatchCreate(_ context.Context, logs []*model.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, log := range logs {
		if log == nil {
			continue
		}
		r.scope++
		log.ID = r.scope
		if log.CreatedAt.IsZero() {
			log.CreatedAt = now
		}
		r.logs = append(r.logs, log)
	}
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
	// 分页：AuditLogFilter 用 Page/PageSize 表示分页，此处转换为 offset/limit。
	offset := 0
	if filter.Page > 0 {
		offset = (filter.Page - 1) * filter.PageSize
	}
	limit := filter.PageSize
	if offset > len(result) {
		result = nil
	} else {
		result = result[offset:]
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, total, nil
}

var _ AuditLogRepository = (*MemoryAuditLogRepository)(nil)
