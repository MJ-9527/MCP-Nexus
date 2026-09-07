package repository

import (
	"MCP-Nexus/model"
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type ServerRepository interface {
	Create(context.Context, *model.MCPServer) error
	FindByName(context.Context, string) (*model.MCPServer, error)
	FindByEndpoint(context.Context, string) (*model.MCPServer, error)
	List(ctx context.Context) ([]*model.MCPServer, error)
	FindByID(ctx context.Context, id int64) (*model.MCPServer, error)
}
