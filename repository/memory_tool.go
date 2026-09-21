package repository

import (
	"context"
	"strings"
	"sync"

	"MCP-Nexus/model"
)

// MemoryToolRepository 内存版 ToolRepository，用于单元测试与本地开发（无需 PostgreSQL）。
type MemoryToolRepository struct {
	mu     sync.RWMutex
	nextID int64
	tools  map[int64]*model.MCPTool
}

func NewMemoryToolRepository() *MemoryToolRepository {
	return &MemoryToolRepository{nextID: 1, tools: make(map[int64]*model.MCPTool)}
}

// 编译期接口校验：确保 MemoryToolRepository 与 PostgresToolRepository 实现同一契约。
var _ ToolRepository = (*MemoryToolRepository)(nil)

func (r *MemoryToolRepository) Create(_ context.Context, t *model.MCPTool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.tools {
		if x.ServerID == t.ServerID && strings.EqualFold(x.Name, t.Name) {
			return ErrConflict
		}
	}
	if t.ID == 0 {
		t.ID = r.nextID
		r.nextID++
	}
	x := *t
	r.tools[t.ID] = &x
	return nil
}

func (r *MemoryToolRepository) FindByID(_ context.Context, id int64) (*model.MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.tools[id]; ok {
		x := *t
		return &x, nil
	}
	return nil, ErrNotFound
}

func (r *MemoryToolRepository) FindByName(_ context.Context, serverID int64, name string) (*model.MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tools {
		if t.ServerID == serverID && strings.EqualFold(t.Name, name) {
			x := *t
			return &x, nil
		}
	}
	return nil, ErrNotFound
}

// FindPublishedByName 跨 Server 按名称精确查找第一个已发布工具，供网关调用使用。
func (r *MemoryToolRepository) FindPublishedByName(_ context.Context, name string) (*model.MCPTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.tools {
		if t.Published && strings.EqualFold(t.Name, name) {
			x := *t
			return &x, nil
		}
	}
	return nil, ErrNotFound
}

func (r *MemoryToolRepository) List(_ context.Context, filter ToolFilter) ([]*model.MCPTool, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	matched := make([]*model.MCPTool, 0, len(r.tools))
	for _, t := range r.tools {
		if filter.ServerID != nil && t.ServerID != *filter.ServerID {
			continue
		}
		if filter.Name != "" && !strings.Contains(strings.ToLower(t.Name), strings.ToLower(filter.Name)) {
			continue
		}
		if filter.Keyword != "" {
			keyword := strings.ToLower(filter.Keyword)
			if !strings.Contains(strings.ToLower(t.Name), keyword) && !strings.Contains(strings.ToLower(t.Description), keyword) {
				continue
			}
		}
		if filter.Category != "" && !strings.EqualFold(t.Category, filter.Category) {
			continue
		}
		if len(filter.Tags) > 0 && !containsAllTags(t.Tags, filter.Tags) {
			continue
		}
		if filter.Published != nil && t.Published != *filter.Published {
			continue
		}
		if filter.HealthStatus != "" && !strings.EqualFold(t.HealthStatus, filter.HealthStatus) {
			continue
		}
		x := *t
		matched = append(matched, &x)
	}
	total := int64(len(matched))
	if filter.PageSize > 0 && filter.Page > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		if offset >= len(matched) {
			return []*model.MCPTool{}, total, nil
		}
		end := offset + filter.PageSize
		if end > len(matched) {
			end = len(matched)
		}
		matched = matched[offset:end]
	}
	return matched, total, nil
}

func containsAllTags(actual, expected []string) bool {
	for _, want := range expected {
		found := false
		for _, got := range actual {
			if strings.EqualFold(got, want) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (r *MemoryToolRepository) UpdatePublished(_ context.Context, id int64, published bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tools[id]
	if !ok {
		return ErrNotFound
	}
	t.Published = published
	return nil
}

// UpdateSensitivity 更新工具的敏感标记与敏感级别（与 PostgresToolRepository 对齐）。
func (r *MemoryToolRepository) UpdateSensitivity(_ context.Context, id int64, sensitive bool, level *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tools[id]
	if !ok {
		return ErrNotFound
	}
	t.IsSensitive = sensitive
	t.SensitiveLevel = level
	return nil
}
