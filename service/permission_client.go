package service

import (
	"context"

	"MCP-Nexus/repository"
)

// PermissionActionCall 调用类权限的 action 常量。
const PermissionActionCall = "call"

// rolePermissionClient 将 ToolPermissionRepository 适配为 ProxyService 依赖的 PermissionClient。
// 网关只关心"角色 → 可调用工具名集合"，不感知权限存储细节。
type rolePermissionClient struct {
	permissions repository.ToolPermissionRepository
}

// 编译期校验：PermissionClient 接口（定义于 service/proxy.go）。
var _ PermissionClient = (*rolePermissionClient)(nil)

func NewRolePermissionClient(permissions repository.ToolPermissionRepository) PermissionClient {
	return &rolePermissionClient{permissions: permissions}
}

// GetAllowedToolNamesByRole 返回角色可调用（action=call）的工具名集合。
func (c *rolePermissionClient) GetAllowedToolNamesByRole(ctx context.Context, role string) ([]string, error) {
	return c.permissions.ListToolNamesByRole(ctx, role, PermissionActionCall)
}
