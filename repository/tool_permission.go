package repository

import (
	"context"

	"MCP-Nexus/model"
)

type ToolPermissionRepository interface {
	Grant(ctx context.Context, permission *model.ToolPermission) error
	Revoke(ctx context.Context, toolID int64, userID, roleID *int64, action string) error
	ListByTool(ctx context.Context, toolID int64) ([]*model.ToolPermission, error)
	HasPermission(ctx context.Context, userID, toolID int64, action string) (bool, error)
	ConfigureRolePermissions(ctx context.Context, toolID int64, sensitive bool, level *string, permissions []*model.ToolPermission) error
}
