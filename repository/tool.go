package repository

import (
	"MCP-Nexus/model"
	"context"
)

type ToolRepository interface {
	// ListAllTools 返回全部工具原始数据，不做server状态过滤
	ListAllTools(ctx context.Context) ([]*model.MCPTool, error)
	// FindToolByName 根据工具名仅查询工具实体
	FindToolByName(ctx context.Context, toolName string) (*model.MCPTool, error)
}
