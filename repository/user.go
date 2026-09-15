package repository

import (
	"MCP-Nexus/model"
	"context"
)

// UserRepository 用户数据访问接口。
type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
}
