package router

import (
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/config"
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func SetupRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.RequestID())

	r.GET("/health", handler.Health)

	// Repository → Service → Handler 装配（网关不绕过 Repository，不直接操作数据库）
	serverRepo := repository.NewPostgresServerRepository(pool)
	serverService := service.NewServerService(serverRepo)
	serverHealthService := service.NewServerHealthService(serverRepo, client.NewHealthClient(5*time.Second))
	toolRepo := repository.NewPostgresToolRepository(pool)
	toolService := service.NewToolService(toolRepo, serverRepo)
	userRepo := repository.NewPostgresUserRepository(pool)
	permissionRepo := repository.NewPostgresToolPermissionRepository(pool)
	permissionService := service.NewToolPermissionService(permissionRepo, toolRepo, userRepo)
	auditRepo := repository.NewPostgresAuditLogRepository(pool)
	auditService := service.NewAuditLogService(auditRepo)

	serverRegisterHandler := handler.NewRegisterHandler(serverService)
	serverQueryHandler := handler.NewServerQueryHandler(serverService)
	serverStatusHandler := handler.NewServerStatusHandler(serverService)
	serverHealthHandler := handler.NewServerHealthHandler(serverHealthService)
	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)
	permissionHandler := handler.NewToolPermissionHandler(permissionService)
	auditHandler := handler.NewAuditLogHandler(auditService)
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTTTL)
	authHandler := handler.NewAuthHandler(authService)

	api := r.Group("/api")
	api.POST("/auth/login", authHandler.Login)

	// B6：/api 管理面强制 JWT 认证；管理写操作仅限 admin 角色
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(cfg.JWTSecret))

	servers := protected.Group("/servers")
	servers.GET("", serverQueryHandler.ListServers)
	servers.GET("/:id", serverQueryHandler.GetServer)
	manage := protected.Group("")
	manage.Use(middleware.RequireRole("admin"))
	serversManage := manage.Group("/servers")
	serversManage.POST("", serverRegisterHandler.RegisterServer)
	serversManage.POST("/:id/activate", serverStatusHandler.ActivateServer)
	serversManage.POST("/:id/offline", serverStatusHandler.OfflineServer)
	serversManage.POST("/:id/health-check", serverHealthHandler.CheckServer)

	tools := protected.Group("/tools")
	tools.GET("", toolQueryHandler.ListTools)
	tools.GET("/:id", toolQueryHandler.GetTool)
	toolsManage := manage.Group("/tools")
	toolsManage.POST("", toolRegisterHandler.RegisterTool)
	toolsManage.POST("/:id/publish", toolPublishHandler.PublishTool)
	toolsManage.POST("/:id/offline", toolPublishHandler.OfflineTool)

	permissions := toolsManage.Group("/:id/permissions")
	permissions.POST("", permissionHandler.GrantPermission)
	permissions.DELETE("", permissionHandler.RevokePermission)
	permissions.GET("", permissionHandler.ListPermissions)
	permissions.GET("/check", permissionHandler.CheckPermission)

	auditLogs := manage.Group("/audit-logs")
	auditLogs.POST("", auditHandler.Create)
	auditLogs.GET("", auditHandler.List)

	// MCP 网关代理（B1）：工具发现 + 调用转发，经统一调用链（权限过滤 → 状态检查 → 转发）
	// B5：/mcp 强制 JWT 认证，角色与用户 ID 由令牌注入，不再信任 X-Role
	// B7：JWT 之后挂 Redis 令牌桶限流（角色限额 admin 600/dev 300/agent 120 每分钟），Redis 不可用降级放行
	permissionClient := service.NewRolePermissionClient(permissionRepo)
	proxySvc := service.NewProxyService(serverRepo, toolRepo, permissionClient)
	proxyHandler := handler.NewProxyHandler(proxySvc)
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	limiter := middleware.NewRateLimiter(rdb, middleware.DefaultRoleLimits())
	mcp := r.Group("/mcp")
	mcp.Use(middleware.JWTAuth(cfg.JWTSecret))
	mcp.Use(limiter.Middleware())
	{
		mcp.GET("/tools", proxyHandler.ListTools)
		mcp.POST("/tools/:toolName/call", proxyHandler.CallTool)
	}

	return r
}
