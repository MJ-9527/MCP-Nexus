package service

import (
	"context"
	"sort"

	"MCP-Nexus/repository"
)

// RBAC 操作权限常量（B6）：区分查看、调用、发布、管理四类操作。
const (
	PermissionActionView    = "view"
	PermissionActionCall    = "call"
	PermissionActionPublish = "publish"
	PermissionActionManage  = "manage"
)

// PermissionClient 对接权限服务，查询「角色授权 ∪ 用户直授」在指定操作下允许的工具名集合。
// userID 传 0 表示仅按角色查询；role 传空串表示仅按用户直授查询。
type PermissionClient interface {
	GetAllowedToolNames(ctx context.Context, role string, userID int64, action string) ([]string, error)
}

// rolePermissionClient 将 ToolPermissionRepository 适配为 ProxyService 依赖的 PermissionClient。
// 网关只关心"principal → 可操作工具名集合"，不感知权限存储细节。
type rolePermissionClient struct {
	permissions repository.ToolPermissionRepository
}

// 编译期校验：PermissionClient 接口（定义于 service/proxy.go）。
var _ PermissionClient = (*rolePermissionClient)(nil)

func NewRolePermissionClient(permissions repository.ToolPermissionRepository) PermissionClient {
	return &rolePermissionClient{permissions: permissions}
}

// GetAllowedToolNames 合并角色与用户直授两个维度的授权并去重排序。
func (c *rolePermissionClient) GetAllowedToolNames(ctx context.Context, role string, userID int64, action string) ([]string, error) {
	seen := make(map[string]bool)
	names := make([]string, 0)
	appendNames := func(list []string) {
		for _, name := range list {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	if role != "" {
		byRole, err := c.permissions.ListToolNamesByRole(ctx, role, action)
		if err != nil {
			return nil, err
		}
		appendNames(byRole)
	}
	if userID > 0 {
		byUser, err := c.permissions.ListToolNamesByUser(ctx, userID, action)
		if err != nil {
			return nil, err
		}
		appendNames(byUser)
	}
	sort.Strings(names)
	return names, nil
}
