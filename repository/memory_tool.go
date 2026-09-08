package repository

import (
	"context"
	"sync"

	"MCP-Nexus/model"
)

type MemoryToolRepository struct {
	mu      sync.RWMutex
	toolMap map[string]*model.MCPTool // key:toolName
}

func NewMemoryToolRepository() *MemoryToolRepository {
	return &MemoryToolRepository{
		toolMap: make(map[string]*model.MCPTool),
	}
}

// 编译期接口校验
var _ ToolRepository = (*MemoryToolRepository)(nil)

func (m *MemoryToolRepository) ListAllTools(_ context.Context) ([]*model.MCPTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*model.MCPTool, 0, len(m.toolMap))
	for _, t := range m.toolMap {
		out = append(out, t)
	}
	return out, nil
}

func (m *MemoryToolRepository) FindToolByName(_ context.Context, toolName string) (*model.MCPTool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tool, ok := m.toolMap[toolName]
	if !ok {
		return nil, ErrNotFound
	}
	return tool, nil
}

// 供注册模块调用，写入工具元数据
func (m *MemoryToolRepository) RegisterTool(tool *model.MCPTool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolMap[tool.Name] = tool
}

// 供注册模块调用，删除工具
func (m *MemoryToolRepository) RemoveTool(toolName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.toolMap, toolName)
}
