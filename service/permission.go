package service

import (
	"context"
	"errors"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidPermission = errors.New("invalid permission")
	ErrPermissionExists  = errors.New("permission already exists")
	ErrPermissionNotFound = errors.New("permission not found")
)

type PermissionService struct {
	perms repository.ToolPermissionRepository
}

func NewPermissionService(perms repository.ToolPermissionRepository) *PermissionService {
	return &PermissionService{perms: perms}
}

func (s *PermissionService) Grant(ctx context.Context, p *model.ToolPermission) error {
	if s == nil || s.perms == nil || p == nil {
		return ErrInvalidPermission
	}
	if p.ToolID <= 0 || p.Action == "" {
		return ErrInvalidPermission
	}
	if (p.UserID == nil) == (p.RoleID == nil) {
		return ErrInvalidPermission
	}
	if err := s.perms.Grant(ctx, p); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return ErrPermissionExists
		}
		if errors.Is(err, repository.ErrNotFound) {
			return ErrPermissionNotFound
		}
		return err
	}
	return nil
}

func (s *PermissionService) Revoke(ctx context.Context, toolID int64, userID, roleID *int64, action string) error {
	if s == nil || s.perms == nil || toolID <= 0 || action == "" {
		return ErrInvalidPermission
	}
	if (userID == nil) == (roleID == nil) {
		return ErrInvalidPermission
	}
	if err := s.perms.Revoke(ctx, toolID, userID, roleID, action); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrPermissionNotFound
		}
		return err
	}
	return nil
}

func (s *PermissionService) ListByTool(ctx context.Context, toolID int64) ([]*model.ToolPermission, error) {
	if s == nil || s.perms == nil || toolID <= 0 {
		return nil, ErrInvalidPermission
	}
	return s.perms.ListByTool(ctx, toolID)
}

func (s *PermissionService) HasPermission(ctx context.Context, userID, toolID int64, action string) (bool, error) {
	if s == nil || s.perms == nil || userID <= 0 || toolID <= 0 || action == "" {
		return false, ErrInvalidPermission
	}
	return s.perms.HasPermission(ctx, userID, toolID, action)
}
