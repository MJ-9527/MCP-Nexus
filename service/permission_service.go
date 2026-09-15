package service

import (
	"context"

	"MCP-Nexus/repository"
)

// PermissionService 实现权限查询，并通过适配器模式满足 permission.PermissionClient 接口。
type PermissionService struct {
	repo repository.PermissionRepository
}

func NewPermissionService(repo repository.PermissionRepository) *PermissionService {
	return &PermissionService{repo: repo}
}

// GetAllowedToolNamesByRole 查询角色被允许的工具列表（"*" 表示全部放行）。
func (s *PermissionService) GetAllowedToolNamesByRole(ctx context.Context, role string) ([]string, error) {
	tools, err := s.repo.FindToolsByRole(ctx, role)
	if err != nil {
		return nil, err
	}
	if len(tools) == 0 {
		return []string{}, nil
	}
	return tools, nil
}

// IsAllowed 判断某角色是否被允许调用某工具（支持 "*" 通配）。
func (s *PermissionService) IsAllowed(ctx context.Context, role, toolName string) (bool, error) {
	tools, err := s.GetAllowedToolNamesByRole(ctx, role)
	if err != nil {
		return false, err
	}
	for _, t := range tools {
		if t == "*" || t == toolName {
			return true, nil
		}
	}
	return false, nil
}

// SetPermission 授予或撤销角色对工具的调用权限（仅管理员调用）。
func (s *PermissionService) SetPermission(ctx context.Context, role, toolName, action string) error {
	if role != "admin" && role != "developer" && role != "agent" {
		return repository.ErrInvalidParameter
	}
	if toolName == "" {
		return repository.ErrInvalidParameter
	}
	if action != "grant" && action != "revoke" {
		return repository.ErrInvalidParameter
	}
	return s.repo.SetPermission(ctx, role, toolName, action)
}
