package repository

import "context"

// PermissionRepository 角色-工具权限映射数据访问接口。
type PermissionRepository interface {
	// FindToolsByRole 查询指定角色被允许调用的工具名列表（"*" 表示全部放行）。
	FindToolsByRole(ctx context.Context, role string) ([]string, error)
	// SetPermission 授予或撤销角色对工具的调用权限。
	// toolName 为 "*" 表示全部放行；action 为 "grant" 或 "revoke"。
	SetPermission(ctx context.Context, role, toolName, action string) error
}
