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
	// ListToolNamesByRole 查询某角色在指定操作下被授权的工具名集合（仅统计已发布工具），供网关 RBAC 过滤。
	ListToolNamesByRole(ctx context.Context, roleName string, action string) ([]string, error)
	// ListToolNamesByUser 查询某用户直接授权（user_id 维度）在指定操作下的工具名集合（仅统计已发布工具）。
	ListToolNamesByUser(ctx context.Context, userID int64, action string) ([]string, error)
	ConfigureRolePermissions(ctx context.Context, toolID int64, sensitive bool, level *string, permissions []*model.ToolPermission) error
}
