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
