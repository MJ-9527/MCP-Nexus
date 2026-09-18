package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"MCP-Nexus/config"
	"MCP-Nexus/router"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx)
	if err != nil {
		log.Fatal("连接 PostgreSQL 失败：", err)
	}
	defer pool.Close()

	r := router.SetupRouter(pool, cfg)

	// B14：监听退出信号，优雅关闭审计批量写入器，保证已入队记录落地。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("收到信号 %v，开始优雅关闭...", sig)
		if router.AuditWriterForShutdown != nil {
			router.AuditWriterForShutdown.Stop()
		}
		os.Exit(0)
	}()

	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
