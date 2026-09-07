package service

import (
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
	"context"
	"errors"
	"net/url"
	"strings"
)

var (
	ErrInvalidServer = errors.New("invalid server registration")
	ErrServerExists  = errors.New("server already exists")
)

func (s *ServerService) Register(ctx context.Context, req model.RegisterServerRequest) (*model.MCPServer, error) {
	if s == nil || s.servers == nil {
		return nil, ErrInvalidServer
	}
	name := strings.TrimSpace(req.Name)
	endpoint := strings.TrimRight(strings.TrimSpace(req.Endpoint), "/")
	version := strings.TrimSpace(req.Version)
	u, e := url.ParseRequestURI(endpoint)
	if name == "" || version == "" || e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, ErrInvalidServer
	}
	if x, e := s.servers.FindByName(ctx, name); e == nil && x != nil {
		return nil, ErrServerExists
	}
	if x, e := s.servers.FindByEndpoint(ctx, endpoint); e == nil && x != nil {
		return nil, ErrServerExists
	}
	now := s.now()
	server := &model.MCPServer{Name: name, Description: strings.TrimSpace(req.Description), Endpoint: endpoint, Version: version, OwnerID: req.OwnerID, Status: "draft", HealthStatus: "unknown", CreatedAt: now, UpdatedAt: now}
	if e := s.servers.Create(ctx, server); e != nil {
		if errors.Is(e, repository.ErrConflict) {
			return nil, ErrServerExists
		}
		return nil, e
	}
	return server, nil
}
