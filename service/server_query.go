package service

import (
	"MCP-Nexus/model"

	"context"
)

func (s *ServerService) List(ctx context.Context) ([]*model.MCPServer, error) {
	if s == nil || s.servers == nil {
		return nil, ErrInvalidServer
	}
	return s.servers.List(ctx)
}

func (s *ServerService) GetByID(ctx context.Context, id int64) (*model.MCPServer, error) {
	if s == nil || s.servers == nil {
		return nil, ErrInvalidServer
	}
	return s.servers.FindByID(ctx, id)
}
