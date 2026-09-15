package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"MCP-Nexus/repository"
)

var ErrInvalidServerStatusTransition = errors.New("invalid server status transition")

// HealthCheckResult 健康检查返回结果。
type HealthCheckResult struct {
	ServerID     int64     `json:"server_id"`
	HealthStatus string    `json:"health_status"`
	LatencyMS    int64     `json:"latency_ms"`
	CheckedAt    time.Time `json:"checked_at"`
}

func (s *ServerService) Activate(ctx context.Context, id int64) error {
	return s.setStatus(ctx, id, "active")
}

func (s *ServerService) Offline(ctx context.Context, id int64) error {
	return s.setStatus(ctx, id, "offline")
}

// HealthCheck 对指定 Server 执行一次健康检查：GET {endpoint}/health（2s 超时），
// 根据响应更新 health_status 与 last_health_check_at。
func (s *ServerService) HealthCheck(ctx context.Context, id int64) (*HealthCheckResult, error) {
	if s == nil || s.servers == nil || id <= 0 {
		return nil, ErrInvalidServer
	}
	server, err := s.servers.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 2 * time.Second}
	url := strings.TrimRight(server.Endpoint, "/") + "/health"
	start := time.Now()
	resp, err := client.Get(url)
	latency := time.Since(start).Milliseconds()
	checkedAt := s.now()

	var healthStatus string
	switch {
	case err != nil:
		healthStatus = "offline" // 超时、连接拒绝 → 离线
	case resp.StatusCode == http.StatusOK:
		healthStatus = "online"
	default:
		healthStatus = "degraded" // 非 200 → 异常
	}
	if resp != nil {
		resp.Body.Close()
	}

	if err := s.servers.UpdateHealth(ctx, id, healthStatus, checkedAt); err != nil {
		return nil, err
	}
	return &HealthCheckResult{
		ServerID:     id,
		HealthStatus: healthStatus,
		LatencyMS:    latency,
		CheckedAt:    checkedAt,
	}, nil
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
