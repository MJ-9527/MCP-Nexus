package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"MCP-Nexus/config"
	"MCP-Nexus/repository"
	"MCP-Nexus/router"
	"MCP-Nexus/service"

	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()
	rootCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := config.NewPostgresPool(rootCtx)
	if err != nil {
		log.Fatal("连接 PostgreSQL 失败：", err)
	}
	defer pool.Close()

	// Redis（限流）
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := rdb.Ping(rootCtx).Err(); err != nil {
		log.Println("警告：Redis 连接失败，限流将降级放行：", err)
	}

	// ClickHouse（审计）
	auditRepo, err := repository.NewClickHouseAuditRepository(cfg.ClickHouseAddr)
	if err != nil {
		log.Println("警告：ClickHouse 连接失败，审计日志将丢失：", err)
	}
	if auditRepo != nil {
		if err := auditRepo.EnsureSchema(rootCtx); err != nil {
			log.Println("警告：ClickHouse 建表失败：", err)
		}
	}

	// Repository → Service 注入
	serverRepo := repository.NewPostgresServerRepository(pool)
	userRepo := repository.NewPostgresUserRepository(pool)
	permissionRepo := repository.NewPostgresPermissionRepository(pool)

	serverService := service.NewServerService(serverRepo)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTTTL)
	permissionService := service.NewPermissionService(permissionRepo)

	var auditService *service.AuditService
	if auditRepo != nil {
		auditService = service.NewAuditService(auditRepo)
		go auditService.Start(rootCtx)
	}

	// 启动定时健康检查
	serverService.StartPeriodicHealthCheck(rootCtx, 60*time.Second)

	// 启动路由
	r := router.SetupRouter(router.RouterDeps{
		Pool:              pool,
		ServerService:     serverService,
		AuthService:       authService,
		PermissionService: permissionService,
		AuditService:      auditService,
		Redis:             rdb,
		ClickHouse:        auditRepo,
	})

	go func() {
		if err := r.Run(":" + cfg.Port); err != nil {
			log.Fatal(err)
		}
	}()

	log.Printf("MCP-Nexus 网关启动，监听 :%s", cfg.Port)
	<-rootCtx.Done()
	log.Println("收到退出信号，正在关闭...")
}
