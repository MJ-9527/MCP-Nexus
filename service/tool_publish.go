package service

import (
	"context"
	"errors"

	"MCP-Nexus/repository"
)

func (s *ToolService) SetPublished(ctx context.Context, id int64, published bool) error {
	if s == nil || s.tools == nil || id <= 0 {
		return ErrInvalidTool
	}
	if err := s.tools.UpdatePublished(ctx, id, published); errors.Is(err, repository.ErrNotFound) {
		return ErrToolNotFound
	} else {
		return err
	}
}
