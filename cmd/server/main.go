package main

import (
	"MCP-Nexus/config"
	"MCP-Nexus/repository"
	"MCP-Nexus/router"
	"MCP-Nexus/service"
	"context"
	"log"
	"os"
	"time"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx)
	if err != nil {
		log.Fatal("连接 PostgreSQL 失败：", err)
	}
	defer pool.Close()

	var analytics *service.AsyncAuditAnalyticsSink
	clickhouseConn, clickhouseErr := config.NewClickHouse(ctx, cfg)
	if clickhouseErr != nil {
		log.Printf("连接 ClickHouse 失败，分析审计将禁用但主服务继续运行：%v", clickhouseErr)
	} else {
		defer clickhouseConn.Close()
		schema, readErr := os.ReadFile("db/clickhouse/001_a13_audit_logs.sql")
		if readErr != nil {
			log.Printf("读取 ClickHouse schema 失败：%v", readErr)
		} else if execErr := clickhouseConn.Exec(ctx, string(schema)); execErr != nil {
			log.Printf("初始化 ClickHouse schema 失败：%v", execErr)
		} else {
			analytics = service.NewAsyncAuditAnalyticsSink(repository.NewClickHouseAuditAnalyticsSink(clickhouseConn), 1000, 100, time.Second)
			defer analytics.Close()
			log.Printf("ClickHouse 已连接：%s/%s", cfg.ClickHouseAddr, cfg.ClickHouseDatabase)
		}
	}

	r := router.SetupRouterWithAnalytics(pool, cfg, clickhouseConn, analytics)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
