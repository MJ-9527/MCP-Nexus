package main

import (
	"MCP-Nexus/config"
	"MCP-Nexus/router"
	"context"
	"log"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx)
	if err != nil {
		log.Fatal("连接 PostgreSQL 失败：", err)
	}
	defer pool.Close()

	r := router.SetupRouter(pool)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
