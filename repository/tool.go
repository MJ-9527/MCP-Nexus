package repository

import (
	"MCP-Nexus/model"
	"context"
)

type ToolFilter struct {
	ServerID     *int64
	Name         string
	Keyword      string
	Category     string
	Tags         []string
	Published    *bool
	Status       string
	HealthStatus string
	Sort         string
	Page         int
	PageSize     int
}

type ToolRepository interface {
	Create(ctx context.Context, tool *model.MCPTool) error
	FindByID(ctx context.Context, id int64) (*model.MCPTool, error)
	FindByName(ctx context.Context, serverID int64, name string) (*model.MCPTool, error)
	List(ctx context.Context, filter ToolFilter) ([]*model.MCPTool, int64, error)
	UpdatePublished(ctx context.Context, id int64, published bool) error
	UpdateSensitivity(ctx context.Context, id int64, sensitive bool, level *string) error
}
