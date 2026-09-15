package repository

import (
	"context"

	"MCP-Nexus/model"
)

type ToolFilter struct {
	ServerID     *int64
	Name         string
	Category     string
	Published    *bool
	HealthStatus string
	// ExcludeOffline 排除 health_status='offline' 的工具（市场列表用）
	ExcludeOffline bool
	// Keyword 关键词搜索（匹配 name 或 description）
	Keyword string
	// Page/PageSize 分页（从 1 开始；0 表示不分页）
	Page     int
	PageSize int
}

// Offset 计算偏移量，未启用分页时返回 0
func (f ToolFilter) Offset() int {
	if f.Page <= 1 {
		return 0
	}
	return (f.Page - 1) * f.PageSize
}

type ToolRepository interface {
	Create(ctx context.Context, tool *model.MCPTool) error
	FindByID(ctx context.Context, id int64) (*model.MCPTool, error)
	FindByName(ctx context.Context, serverID int64, name string) (*model.MCPTool, error)
	// FindPublishedByName 跨 Server 按名称精确查找第一个已发布工具，供网关调用使用。
	FindPublishedByName(ctx context.Context, name string) (*model.MCPTool, error)
	List(ctx context.Context, filter ToolFilter) ([]*model.MCPTool, error)
	// Count 按相同过滤条件统计总数（市场分页用）
	Count(ctx context.Context, filter ToolFilter) (int64, error)
	UpdatePublished(ctx context.Context, id int64, published bool) error
}
