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

func SetupRouter(pool *pgxpool.Pool, cfg config.Config, analytics ...*service.AsyncAuditAnalyticsSink) *gin.Engine {
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
	ratingRepo := repository.NewPostgresToolRatingRepository(pool)
	ratingService := service.NewToolRatingService(ratingRepo, toolRepo)
	adaptationRepo := repository.NewPostgresAdaptationTaskRepository(pool)
	adaptationService := service.NewAdaptationService(adaptationRepo, toolRepo)
	auditRepo := repository.NewPostgresAuditLogRepository(pool)
	auditService := service.NewAuditLogService(auditRepo)
	if len(analytics) > 0 && analytics[0] != nil {
		auditService.SetAnalytics(analytics[0])
	}

	serverRegisterHandler := handler.NewRegisterHandler(serverService)
	serverQueryHandler := handler.NewServerQueryHandler(serverService)
	serverStatusHandler := handler.NewServerStatusHandler(serverService)
	serverHealthHandler := handler.NewServerHealthHandler(serverHealthService)
	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)
	permissionHandler := handler.NewToolPermissionHandler(permissionService)
	ratingHandler := handler.NewToolRatingHandler(ratingService)
	adaptationHandler := handler.NewAdaptationHandler(adaptationService)
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
	manage.Use(middleware.RequireRole(middleware.RoleAdmin))
	serversManage := manage.Group("/servers")
	serversManage.POST("", serverRegisterHandler.RegisterServer)
	serversManage.POST("/:id/activate", serverStatusHandler.ActivateServer)
	serversManage.POST("/:id/offline", serverStatusHandler.OfflineServer)
	serversManage.POST("/:id/health-check", serverHealthHandler.CheckServer)

	tools := protected.Group("/tools")
	tools.GET("", toolQueryHandler.ListTools)
	tools.GET("/:id", toolQueryHandler.GetTool)
	tools.GET("/:id/reviews", ratingHandler.List)
	tools.POST("/:id/reviews", ratingHandler.Create)
	tools.GET("/:id/adaptation-tasks", adaptationHandler.List)
	tools.POST("/:id/adaptation-tasks", adaptationHandler.Create)
	tools.GET("/adaptation-tasks/:task_id", adaptationHandler.Get)
	tools.PATCH("/adaptation-tasks/:task_id", adaptationHandler.UpdateStatus)
	toolsManage := manage.Group("/tools")
	toolsManage.POST("", toolRegisterHandler.RegisterTool)
	toolsManage.POST("/:id/publish", toolPublishHandler.PublishTool)
	toolsManage.POST("/:id/offline", toolPublishHandler.OfflineTool)

	permissions := toolsManage.Group("/:id/permissions")
	permissions.POST("", permissionHandler.GrantPermission)
	permissions.DELETE("", permissionHandler.RevokePermission)
	permissions.GET("", permissionHandler.ListPermissions)
	permissions.GET("/check", permissionHandler.CheckPermission)

	// 审计日志（B8）：规范路径与旧路径都经过 JWT 和平台管理员鉴权。
	audit := manage.Group("/audit/logs")
	audit.POST("", auditHandler.Create)
	audit.GET("", auditHandler.List)
	auditLegacy := manage.Group("/audit-logs")
	auditLegacy.POST("", auditHandler.Create)
	auditLegacy.GET("", auditHandler.List)

	// MCP 网关代理（B1）：工具发现 + 调用转发，经统一调用链（权限过滤 → 状态检查 → 转发）
	// B5：/mcp 强制 JWT 认证，角色与用户 ID 由令牌注入，不再信任 X-Role
	// B7：JWT 之后挂 Redis 令牌桶限流（platform_admin 600/tool_developer 300/agent_caller 120 每分钟），Redis 不可用降级放行
	permissionClient := service.NewRolePermissionClient(permissionRepo)
	proxySvc := service.NewProxyService(serverRepo, toolRepo, permissionClient)
	proxySvc.SetAudit(auditService) // B8：调用链审计埋点
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

	// B9：MCP 接入配置生成（登录账号获取自己的接入指引与可用工具清单）
	mcpConfigService := service.NewMcpConfigService(proxySvc, cfg.JWTTTL.String())
	mcpConfigHandler := handler.NewMcpConfigHandler(mcpConfigService, cfg.PublicBaseURL)
	protected.GET("/mcp-config", mcpConfigHandler.GetMcpConfig)

	return r
}
