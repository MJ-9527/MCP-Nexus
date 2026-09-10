package service

import (
	"context"
	"errors"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

func (s *ToolService) List(ctx context.Context, filter repository.ToolFilter) ([]*model.MCPTool, error) {
	if s == nil || s.tools == nil {
		return nil, ErrInvalidTool
	}
	return s.tools.List(ctx, filter)
}

func (s *ToolService) GetByID(ctx context.Context, id int64) (*model.MCPTool, error) {
	if s == nil || s.tools == nil || id <= 0 {
		return nil, ErrInvalidTool
	}
	tool, err := s.tools.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrToolNotFound
	}
	return tool, err
}
