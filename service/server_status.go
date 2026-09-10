package service

import (
	"context"
	"errors"

	"MCP-Nexus/repository"
)

var ErrInvalidServerStatusTransition = errors.New("invalid server status transition")

func (s *ServerService) Activate(ctx context.Context, id int64) error {
	return s.setStatus(ctx, id, "active")
}

func (s *ServerService) Offline(ctx context.Context, id int64) error {
	return s.setStatus(ctx, id, "offline")
}

func (s *ServerService) setStatus(ctx context.Context, id int64, next string) error {
	if s == nil || s.servers == nil || id <= 0 {
		return ErrInvalidServer
	}
	server, err := s.servers.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return repository.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !canChangeServerStatus(server.Status, next) {
		return ErrInvalidServerStatusTransition
	}
	return s.servers.UpdateStatus(ctx, id, next)
}

func canChangeServerStatus(current, next string) bool {
	return (current == "draft" && next == "active") ||
		(current == "active" && next == "offline") ||
		(current == "offline" && next == "active")
}
