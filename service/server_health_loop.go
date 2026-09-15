package service

import (
	"context"
	"log"
	"time"
)

// StartPeriodicHealthCheck 启动后台 goroutine，每隔 interval 拉取所有 Server 执行一次健康检查。
// 应在 main 中以 go 启动，传入根 ctx，服务退出时取消即可停止循环。
func (s *ServerService) StartPeriodicHealthCheck(ctx context.Context, interval time.Duration) {
	go func() {
		// 启动时立即跑一次，再进入固定间隔
		s.runHealthCheckBatch(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("定时健康检查已停止")
				return
			case <-ticker.C:
				s.runHealthCheckBatch(ctx)
			}
		}
	}()
}

// runHealthCheckBatch 拉取所有 Server，逐个调用 HealthCheck，失败只记录日志不中断循环。
func (s *ServerService) runHealthCheckBatch(ctx context.Context) {
	servers, err := s.servers.List(ctx)
	if err != nil {
		log.Printf("[health-check] 拉取 Server 列表失败: %v", err)
		return
	}
	if len(servers) == 0 {
		return
	}
	for _, srv := range servers {
		// 已下线 Server 跳过，避免无意义探测
		if srv.Status == "offline" {
			continue
		}
		result, err := s.HealthCheck(ctx, srv.ID)
		if err != nil {
			log.Printf("[health-check] server id=%d name=%s 检查失败: %v", srv.ID, srv.Name, err)
			continue
		}
		log.Printf("[health-check] server id=%d name=%s → %s (latency=%dms)",
			srv.ID, srv.Name, result.HealthStatus, result.LatencyMS)
	}
}
