package handler

<<<<<<< HEAD
// health.go — Health handler is defined in common.go.
=======
import (
	"context"
	"time"

	"MCP-Nexus/client"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler 处理网关自身健康检查，同时探测核心依赖状态。
type HealthHandler struct {
	pool         *pgxpool.Pool
	healthClient *client.HealthClient
	services     []DownstreamService
}

// DownstreamService 描述一个需要探测的下游服务。
type DownstreamService struct {
	Name     string
	Endpoint string
}

// NewHealthHandler 创建网关健康检查处理器。
// services 是需要探测的下游服务列表（如 demo-service、skills-adapter）。
func NewHealthHandler(pool *pgxpool.Pool, healthClient *client.HealthClient, services []DownstreamService) *HealthHandler {
	return &HealthHandler{
		pool:         pool,
		healthClient: healthClient,
		services:     services,
	}
}

// Health 返回网关自身状态及核心依赖健康状态。
// 只要网关本身可响应，就返回 200；依赖异常时在 dependencies 中标记为 unhealthy，整体 status 为 degraded。
func (h *HealthHandler) Health(c *gin.Context) {
	ctx := context.Background()
	if c.Request != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
	}

	result := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"dependencies": map[string]any{
			"postgres": h.checkPostgres(ctx),
		},
	}

	deps := result["dependencies"].(map[string]any)
	hasUnhealthy := false
	for _, svc := range h.services {
		deps[svc.Name] = h.checkService(ctx, svc.Endpoint)
	}

	for _, v := range deps {
		if d, ok := v.(map[string]any); ok && d["status"] != "healthy" {
			hasUnhealthy = true
		}
	}
	if hasUnhealthy {
		result["status"] = "degraded"
	}

	respondSuccess(c, result)
}

func (h *HealthHandler) checkPostgres(ctx context.Context) map[string]any {
	if h.pool == nil {
		return map[string]any{"status": "unknown", "latency_ms": 0}
	}
	start := time.Now()
	if err := h.pool.Ping(ctx); err != nil {
		return map[string]any{
			"status":     "unhealthy",
			"error":      err.Error(),
			"latency_ms": time.Since(start).Milliseconds(),
		}
	}
	return map[string]any{
		"status":     "healthy",
		"latency_ms": time.Since(start).Milliseconds(),
	}
}

func (h *HealthHandler) checkService(ctx context.Context, endpoint string) map[string]any {
	res := h.healthClient.Check(ctx, endpoint)
	if res.Err != nil {
		return map[string]any{
			"status":     "unhealthy",
			"error":      res.Err.Error(),
			"latency_ms": res.Latency.Milliseconds(),
		}
	}
	return map[string]any{
		"status":     "healthy",
		"latency_ms": res.Latency.Milliseconds(),
	}
}

// Health 兼容旧的包级函数签名（无依赖检查）。
// 推荐在 router 中使用 NewHealthHandler 以获得完整依赖健康状态。
func Health(c *gin.Context) {
	respondSuccess(c, gin.H{
		"status": "ok",
	})
}
>>>>>>> origin/pull-request
