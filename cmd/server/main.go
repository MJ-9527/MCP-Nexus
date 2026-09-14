package main

import (
	"context"
	"log"
	"time"

	"MCP-Nexus/config"
	"MCP-Nexus/router"
)

// healthCheckInterval 定时健康检查间隔；可改为 60 * time.Second。
const healthCheckInterval = 30 * time.Second

func main() {
	cfg := config.Load()

	r, healthChecker := router.SetupRouter()

	// 后台健康检查：启动时立即检查一次，之后每 30s 检查一次。
	healthChecker.StartBackground(context.Background(), healthCheckInterval, log.Printf)

	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
