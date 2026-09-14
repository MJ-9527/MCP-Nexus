package service

import (
	"context"
	"errors"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/repository"
)

type HealthCheckResult struct {
	ServerID     int64     `json:"server_id"`
	HealthStatus string    `json:"health_status"`
	LatencyMS    int64     `json:"latency_ms"`
	CheckedAt    time.Time `json:"checked_at"`
}
type ServerHealthService struct {
	servers repository.ServerRepository
	client  *client.HealthClient
	now     func() time.Time
}

func NewServerHealthService(servers repository.ServerRepository, client *client.HealthClient) *ServerHealthService {
	return &ServerHealthService{servers: servers, client: client, now: time.Now}
}
func (s *ServerHealthService) Check(ctx context.Context, id int64) (*HealthCheckResult, error) {
	if s == nil || s.servers == nil || s.client == nil || id <= 0 {
		return nil, ErrInvalidServer
	}
	server, err := s.servers.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	latency, statusCode, checkErr := s.client.Check(ctx, server.Endpoint)
	healthStatus := "online"
	if checkErr != nil {
		healthStatus = "offline"
	} else if statusCode < 200 || statusCode >= 300 {
		healthStatus = "degraded"
	}
	checkedAt := s.now()
	if err := s.servers.UpdateHealth(ctx, id, healthStatus, checkedAt); err != nil {
		return nil, err
	}
	return &HealthCheckResult{ServerID: id, HealthStatus: healthStatus, LatencyMS: latency.Milliseconds(), CheckedAt: checkedAt}, nil
}
