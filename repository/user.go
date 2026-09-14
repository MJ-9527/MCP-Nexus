package repository

import (
	"context"

	"MCP-Nexus/model"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	CreateRole(ctx context.Context, role *model.Role) error
	AssignRole(ctx context.Context, userID, roleID int64) error
}
