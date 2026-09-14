package repository

import (
	"MCP-Nexus/model"
	"context"
)

type ToolFilter struct {
	ServerID     *int64
	Name         string
	Category     string
	Published    *bool
	HealthStatus string
}

type ToolRepository interface {
	Create(ctx context.Context, tool *model.MCPTool) error
	FindByID(ctx context.Context, id int64) (*model.MCPTool, error)
	FindByName(ctx context.Context, serverID int64, name string) (*model.MCPTool, error)
	List(ctx context.Context, filter ToolFilter) ([]*model.MCPTool, error)
	UpdatePublished(ctx context.Context, id int64, published bool) error
}
