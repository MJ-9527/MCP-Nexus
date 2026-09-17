package service

import (
	"context"
	"errors"
	"strings"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidPermission  = errors.New("invalid tool permission")
	ErrPermissionExists   = errors.New("tool permission already exists")
	ErrPermissionNotFound = errors.New("tool permission not found")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrInvalidSensitivity = errors.New("invalid sensitivity configuration")
)

type ToolPermissionService struct {
	permissions repository.ToolPermissionRepository
	tools       repository.ToolRepository
	users       repository.UserRepository
}

func NewToolPermissionService(permissions repository.ToolPermissionRepository, tools repository.ToolRepository, users repository.UserRepository) *ToolPermissionService {
	return &ToolPermissionService{permissions: permissions, tools: tools, users: users}
}

func (s *ToolPermissionService) Grant(ctx context.Context, toolID int64, req model.GrantToolPermissionRequest) (*model.ToolPermission, error) {
	if err := s.validate(req, toolID); err != nil {
		return nil, err
	}
	if _, err := s.tools.FindByID(ctx, toolID); errors.Is(err, repository.ErrNotFound) {
		return nil, ErrPermissionNotFound
	} else if err != nil {
		return nil, err
	}
	if req.UserID != nil {
		if _, err := s.users.FindByID(ctx, *req.UserID); errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPermissionNotFound
		} else if err != nil {
			return nil, err
		}
	}
	if req.RoleID != nil {
		if _, err := s.users.FindRoleByID(ctx, *req.RoleID); errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPermissionNotFound
		} else if err != nil {
			return nil, err
		}
	}
	action := strings.TrimSpace(*req.Action)
	permission := &model.ToolPermission{ToolID: toolID, UserID: req.UserID, RoleID: req.RoleID, Action: action}
	if err := s.permissions.Grant(ctx, permission); errors.Is(err, repository.ErrConflict) {
		return nil, ErrPermissionExists
	} else if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrPermissionNotFound
	} else if err != nil {
		return nil, err
	}
	return permission, nil
}

func (s *ToolPermissionService) Revoke(ctx context.Context, toolID int64, req model.GrantToolPermissionRequest) error {
	if err := s.validate(req, toolID); err != nil {
		return err
	}
	if _, err := s.tools.FindByID(ctx, toolID); errors.Is(err, repository.ErrNotFound) {
		return ErrPermissionNotFound
	} else if err != nil {
		return err
	}
	if req.UserID != nil {
		if _, err := s.users.FindByID(ctx, *req.UserID); errors.Is(err, repository.ErrNotFound) {
			return ErrPermissionNotFound
		} else if err != nil {
			return err
		}
	}
	if req.RoleID != nil {
		if _, err := s.users.FindRoleByID(ctx, *req.RoleID); errors.Is(err, repository.ErrNotFound) {
			return ErrPermissionNotFound
		} else if err != nil {
			return err
		}
	}
	action := strings.TrimSpace(*req.Action)
	if err := s.permissions.Revoke(ctx, toolID, req.UserID, req.RoleID, action); errors.Is(err, repository.ErrNotFound) {
		return ErrPermissionNotFound
	} else if err != nil {
		return err
	}
	return nil
}

func (s *ToolPermissionService) ListByTool(ctx context.Context, toolID int64) ([]*model.ToolPermission, error) {
	if s == nil || s.permissions == nil || s.tools == nil || toolID <= 0 {
		return nil, ErrInvalidPermission
	}
	if _, err := s.tools.FindByID(ctx, toolID); errors.Is(err, repository.ErrNotFound) {
		return nil, ErrPermissionNotFound
	} else if err != nil {
		return nil, err
	}
	return s.permissions.ListByTool(ctx, toolID)
}

func (s *ToolPermissionService) Check(ctx context.Context, userID, toolID int64, action string) (bool, error) {
	action = strings.TrimSpace(action)
	if s == nil || s.permissions == nil || s.tools == nil || s.users == nil || userID <= 0 || toolID <= 0 || action == "" {
		return false, ErrInvalidPermission
	}
	if _, err := s.users.FindByID(ctx, userID); errors.Is(err, repository.ErrNotFound) {
		return false, ErrPermissionNotFound
	} else if err != nil {
		return false, err
	}
	if _, err := s.tools.FindByID(ctx, toolID); errors.Is(err, repository.ErrNotFound) {
		return false, ErrPermissionNotFound
	} else if err != nil {
		return false, err
	}
	allowed, err := s.permissions.HasPermission(ctx, userID, toolID, action)
	if err != nil {
		return false, err
	}
	if !allowed {
		return false, ErrPermissionDenied
	}
	return true, nil
}

func (s *ToolPermissionService) Configure(ctx context.Context, toolID int64, req model.ConfigureToolPermissionsRequest) ([]*model.ToolPermission, error) {
	if s == nil || s.permissions == nil || s.tools == nil || s.users == nil || toolID <= 0 {
		return nil, ErrInvalidPermission
	}
	if req.IsSensitive {
		if req.SensitiveLevel == nil {
			return nil, ErrInvalidSensitivity
		}
		level := strings.ToLower(strings.TrimSpace(*req.SensitiveLevel))
		if level != "low" && level != "medium" && level != "high" {
			return nil, ErrInvalidSensitivity
		}
		req.SensitiveLevel = &level
	} else {
		req.SensitiveLevel = nil
	}
	if _, err := s.tools.FindByID(ctx, toolID); errors.Is(err, repository.ErrNotFound) {
		return nil, ErrPermissionNotFound
	} else if err != nil {
		return nil, err
	}
	permissions := make([]*model.ToolPermission, 0)
	seen := make(map[string]struct{})
	for _, item := range req.Permissions {
		name := strings.TrimSpace(item.Role)
		if name == "" {
			return nil, ErrInvalidPermission
		}
		if _, ok := seen[name]; ok {
			return nil, ErrPermissionExists
		}
		seen[name] = struct{}{}
		role, err := s.users.FindRoleByName(ctx, name)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrPermissionNotFound
		}
		if err != nil {
			return nil, err
		}
		if item.CanRead {
			permissions = append(permissions, &model.ToolPermission{ToolID: toolID, RoleID: &role.ID, Action: "read"})
		}
		if item.CanCall {
			permissions = append(permissions, &model.ToolPermission{ToolID: toolID, RoleID: &role.ID, Action: "call"})
		}
	}
	if err := s.permissions.ConfigureRolePermissions(ctx, toolID, req.IsSensitive, req.SensitiveLevel, permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

func (s *ToolPermissionService) Tool(ctx context.Context, toolID int64) (*model.MCPTool, error) {
	tool, err := s.tools.FindByID(ctx, toolID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrPermissionNotFound
	}
	return tool, err
}

func (s *ToolPermissionService) Role(ctx context.Context, roleID int64) (*model.Role, error) {
	role, err := s.users.FindRoleByID(ctx, roleID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrPermissionNotFound
	}
	return role, err
}

func (s *ToolPermissionService) validate(req model.GrantToolPermissionRequest, toolID int64) error {
	if s == nil || s.permissions == nil || s.tools == nil || s.users == nil || toolID <= 0 || req.Action == nil || strings.TrimSpace(*req.Action) == "" {
		return ErrInvalidPermission
	}
	if (req.UserID == nil) == (req.RoleID == nil) {
		return ErrInvalidPermission
	}
	if req.UserID != nil && *req.UserID <= 0 {
		return ErrInvalidPermission
	}
	if req.RoleID != nil && *req.RoleID <= 0 {
		return ErrInvalidPermission
	}
	return nil
}
