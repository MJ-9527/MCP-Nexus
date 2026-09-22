package main

import (
	"MCP-Nexus/config"
	"MCP-Nexus/router"
	"context"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := config.NewPostgresPool(ctx)
	if err != nil {
		log.Printf("警告: 无法连接 PostgreSQL (%v)，将使用内存存储", err)
		log.Println("提示: 使用 docker compose up -d 启动完整环境")
		pool = nil
	}

	var engine *gin.Engine
	if pool != nil {
		engine = router.SetupRouter(pool)
	} else {
		engine = router.SetupRouterFallback()
	}

	log.Printf("网关启动于 :%s", cfg.Port)
	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

