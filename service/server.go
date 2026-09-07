package service

import (
	"time"

	"MCP-Nexus/repository"
)

// ServerService contains MCP Server business use cases.
type ServerService struct {
	servers repository.ServerRepository
	now     func() time.Time
}

func NewServerService(servers repository.ServerRepository) *ServerService {
	return &ServerService{servers: servers, now: time.Now}
}
