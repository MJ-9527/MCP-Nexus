package repository

import (
	"context"
	"sync"

	"MCP-Nexus/model"
)

// MemoryOpenAPISpecRepository 内存版 OpenAPISpecRepository，用于单测与本地开发。
type MemoryOpenAPISpecRepository struct {
	mu     sync.RWMutex
	nextID int64
	specs  map[int64]*model.OpenAPISpec // 按 tool_id 唯一
}

func NewMemoryOpenAPISpecRepository() *MemoryOpenAPISpecRepository {
	return &MemoryOpenAPISpecRepository{nextID: 1, specs: make(map[int64]*model.OpenAPISpec)}
}

// 编译期接口校验。
var _ OpenAPISpecRepository = (*MemoryOpenAPISpecRepository)(nil)

// Save 按 tool_id UPSERT：同一工具重复导入时更新翻译元数据（幂等）。
func (r *MemoryOpenAPISpecRepository) Save(_ context.Context, spec *model.OpenAPISpec) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.specs[spec.ToolID]; ok {
		existing.Method = spec.Method
		existing.PathTemplate = spec.PathTemplate
		existing.Params = spec.Params
		// id/created_at 保留；updated_at 在实际场景由调用方设置或留空
		spec.ID = existing.ID
		spec.CreatedAt = existing.CreatedAt
		return nil
	}
	if spec.ID == 0 {
		spec.ID = r.nextID
		r.nextID++
	}
	c := *spec
	r.specs[spec.ToolID] = &c
	return nil
}

func (r *MemoryOpenAPISpecRepository) FindByToolID(_ context.Context, toolID int64) (*model.OpenAPISpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.specs[toolID]; ok {
		c := *s
		return &c, nil
	}
	return nil, ErrNotFound
}

// ListByServerID 内存版不直接知道 tool→server 映射，故仅返回全部 specs。
// 调用方（仅用于测试场景）通常只在单个 Server 下验证导入结果，此实现足够。
func (r *MemoryOpenAPISpecRepository) ListByServerID(_ context.Context, _ int64) ([]*model.OpenAPISpec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.OpenAPISpec, 0, len(r.specs))
	for _, s := range r.specs {
		c := *s
		out = append(out, &c)
	}
	return out, nil
}
