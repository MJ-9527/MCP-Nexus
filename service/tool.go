package service

import (
	"time"

	"MCP-Nexus/repository"
)

type ToolService struct {
	tools   repository.ToolRepository
	servers repository.ServerRepository
	now     func() time.Time
}

func NewToolService(tools repository.ToolRepository, servers repository.ServerRepository) *ToolService {
	return &ToolService{tools: tools, servers: servers, now: time.Now}
}
