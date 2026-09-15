package service

import (
	"context"
	"errors"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
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
	return s.check(ctx, server), nil
}

// CheckAll 对全部 Server 执行一次健康检查。
func (s *HealthCheckService) CheckAll(ctx context.Context) ([]*HealthCheckResult, error) {
	servers, err := s.servers.List(ctx)
	if err != nil {
		return nil, err
	}
	results := make([]*HealthCheckResult, 0, len(servers))
	for _, server := range servers {
		results = append(results, s.check(ctx, server))
	}
	return results, nil
}

// StartBackground 启动后台健康检查：先立即检查一次，之后每 interval 检查一次，直到 ctx 取消。
func (s *HealthCheckService) StartBackground(ctx context.Context, interval time.Duration, logf func(format string, args ...any)) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if logf == nil {
		logf = func(format string, args ...any) {}
	}

	go func() {
		run := func() {
			defer func() {
				if e := recover(); e != nil {
					logf("health-check panic recovered: %v", e)
				}
			}()

			// 每一轮探测独立超时，避免某个 Server 拖垮整个定时任务。
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			results, err := s.CheckAll(probeCtx)
			if err != nil {
				logf("health-check: list servers failed: %v", err)
				return
			}
			logf("health-check: checked %d servers", len(results))
			for _, r := range results {
				logf("health-check: server_id=%d status=%s latency=%dms", r.ServerID, r.HealthStatus, r.LatencyMS)
			}
		}

		run() // 启动时检查一次
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logf("health-check background stopped")
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

// check 探测单个 Server 并生成结果（不含持久化）。
func (s *HealthCheckService) check(ctx context.Context, server *model.MCPServer) *HealthCheckResult {
	res := s.client.Check(ctx, server.Endpoint)
	checkedAt := s.now()

	result := &HealthCheckResult{
		ServerID:     server.ID,
		HealthStatus: statusFromErr(res.Err),
		LatencyMS:    res.Latency.Milliseconds(),
		CheckedAt:    checkedAt,
	}

	// 持久化 health_status 与 last_health_check_at；latency_ms 随响应返回。
	// 落库失败不影响探测结果（健康探测优先于持久化）。
	_ = s.servers.UpdateHealth(ctx, server.ID, result.HealthStatus, checkedAt)

	return result
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
