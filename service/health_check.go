package service

import (
	"context"
	"errors"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/repository"
)

// ErrHealthServerNotFound 表示健康检查的目标 Server 不存在。
var ErrHealthServerNotFound = errors.New("server not found")

// HealthCheckService 负责探测 MCP Server 的健康状态。
type HealthCheckService struct {
	servers repository.ServerRepository
	client  *client.HealthClient
	now     func() time.Time
}

func NewHealthCheckService(servers repository.ServerRepository, c *client.HealthClient) *HealthCheckService {
	return &HealthCheckService{servers: servers, client: c, now: time.Now}
}

// HealthCheckResult 是一次健康检查的输出。
type HealthCheckResult struct {
	ServerID     int64
	HealthStatus string
	LatencyMS    int64
	CheckedAt    time.Time
}

// HealthCheck 探测指定 Server，返回健康状态与耗时（暂不持久化）。
func (s *HealthCheckService) HealthCheck(ctx context.Context, id int64) (*HealthCheckResult, error) {
	server, err := s.servers.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrHealthServerNotFound
		}
		return nil, err
	}

	res := s.client.Check(ctx, server.Endpoint)
	checkedAt := s.now()

	result := &HealthCheckResult{
		ServerID:     server.ID,
		HealthStatus: statusFromErr(res.Err),
		LatencyMS:    res.Latency.Milliseconds(),
		CheckedAt:    checkedAt,
	}

	// TODO(成员A)：等 ServerRepository 提供 UpdateHealthStatus 方法后，把
	// health_status 与 last_health_check_at 持久化到 PostgreSQL：
	//   if err := s.servers.UpdateHealthStatus(ctx, server.ID, result.HealthStatus, checkedAt); err != nil {
	//       return nil, err
	//   }

	return result, nil
}

// statusFromErr 把探测错误映射为 health_status：
//   - nil              → online（2xx）
//   - ErrUnhealthy     → degraded（非 2xx，服务异常）
//   - 其余（超时/连不上）→ offline
func statusFromErr(err error) string {
	switch {
	case err == nil:
		return "online"
	case errors.Is(err, client.ErrUnhealthy):
		return "degraded"
	default:
		return "offline"
	}
}
