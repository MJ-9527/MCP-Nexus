package repository

import (
	"context"
	"sort"
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

func (r *MemoryToolRepository) List(_ context.Context, filter ToolFilter) ([]*model.MCPTool, error) {
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
		if filter.Category != "" && !strings.EqualFold(t.Category, filter.Category) {
			continue
		}
		if filter.Published != nil && t.Published != *filter.Published {
			continue
		}
		if filter.HealthStatus != "" && t.HealthStatus != filter.HealthStatus {
			continue
		}
		if filter.ExcludeOffline && t.HealthStatus == "offline" {
			continue
		}
		if filter.Keyword != "" {
			kw := strings.ToLower(filter.Keyword)
			if !strings.Contains(strings.ToLower(t.Name), kw) && !strings.Contains(strings.ToLower(t.Description), kw) {
				continue
			}
		}
		x := *t
		matched = append(matched, &x)
	}
	// 稳定排序后按需分页
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID < matched[j].ID })
	if filter.PageSize > 0 {
		start := filter.Offset()
		if start >= len(matched) {
			return []*model.MCPTool{}, nil
		}
		end := start + filter.PageSize
		if end > len(matched) {
			end = len(matched)
		}
		matched = matched[start:end]
	}
	return matched, nil
}

func (r *MemoryToolRepository) Count(_ context.Context, filter ToolFilter) (int64, error) {
	all, err := r.List(context.Background(), filter)
	if err != nil {
		return 0, err
	}
	return int64(len(all)), nil
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

// TestPutTool 直接写入工具用于测试，跳过唯一校验并自动分配 ID。
func (r *MemoryToolRepository) TestPutTool(t *model.MCPTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.ID == 0 {
		t.ID = r.nextID
		r.nextID++
	}
	x := *t
	r.tools[t.ID] = &x
}
