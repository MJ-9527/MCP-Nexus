package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrInvalidTool    = errors.New("invalid tool registration")
	ErrToolExists     = errors.New("tool already exists")
	ErrServerNotFound = errors.New("server not found")
	ErrToolNotFound   = errors.New("tool not found")
)

func (s *ToolService) Register(ctx context.Context, req model.RegisterToolRequest) (*model.MCPTool, error) {
	if s == nil || s.tools == nil || s.servers == nil {
		return nil, ErrInvalidTool
	}
	name, category, version := strings.TrimSpace(req.Name), strings.TrimSpace(req.Category), strings.TrimSpace(req.Version)

	if req.ServerID <= 0 || name == "" || category == "" || version == "" || !json.Valid(req.InputSchema) {
		return nil, ErrInvalidTool
	}
	if _, err := s.servers.FindByID(ctx, req.ServerID); errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServerNotFound
	} else if err != nil {
		return nil, err
	}
	if tool, err := s.tools.FindByName(ctx, req.ServerID, name); err == nil && tool != nil {
		return nil, ErrToolExists
	} else if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	tool := &model.MCPTool{ServerID: req.ServerID, Name: name, Description: strings.TrimSpace(req.Description), Category: category, Tags: req.Tags, InputSchema: req.InputSchema, Version: version, Published: false, HealthStatus: "unknown"}
	if err := s.tools.Create(ctx, tool); errors.Is(err, repository.ErrConflict) {
		return nil, ErrToolExists
	} else if err != nil {
		return nil, err
	}
	return tool, nil
}
