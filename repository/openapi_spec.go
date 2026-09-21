package repository

import (
	"context"

	"MCP-Nexus/model"
)

// OpenAPISpecRepository 存储从 OpenAPI 规范解析出的工具翻译元数据。
// ProxyService 调用前查询此处：命中则走 HTTP 翻译路径，否则按原生 MCP 协议转发。
type OpenAPISpecRepository interface {
	// Save 按 tool_id UPSERT：同一工具重复导入时更新翻译元数据（幂等）。
	Save(ctx context.Context, spec *model.OpenAPISpec) error
	// FindByToolID 按 tool_id 查询单条翻译元数据。不存在返回 ErrNotFound。
	FindByToolID(ctx context.Context, toolID int64) (*model.OpenAPISpec, error)
	// ListByServerID 返回某 Server 下所有 OpenAPI 工具的翻译元数据（JOIN mcp_tools 过滤）。
	ListByServerID(ctx context.Context, serverID int64) ([]*model.OpenAPISpec, error)
}
