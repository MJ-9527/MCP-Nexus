package router

import (
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// RouterDeps 路由依赖（由 main 组装注入）。
type RouterDeps struct {
	Pool              *pgxpool.Pool
	ServerService     *service.ServerService
	AuthService       *service.AuthService
	PermissionService *service.PermissionService
	AuditService      *service.AuditService
	Redis             *redis.Client
	ClickHouse        *repository.ClickHouseAuditRepository
}

// SetupRouter 创建 gin 路由引擎，所有 service 由外部注入。
func SetupRouter(deps RouterDeps) *gin.Engine {
	r := gin.Default()
	r.Use(middleware.RequestID())

	r.GET("/health", handler.Health)

	serverRepo := repository.NewPostgresServerRepository(deps.Pool)
	toolRepo := repository.NewPostgresToolRepository(deps.Pool)
	toolService := service.NewToolService(toolRepo, serverRepo)

	// 第2周：RBAC 内部实现，permissionService 实现 permission.PermissionClient 接口。
	proxyService := service.NewProxyService(serverRepo, toolRepo, deps.PermissionService, deps.AuditService)
	proxyHandler := handler.NewProxyHandler(proxyService)
	authHandler := handler.NewAuthHandler(deps.AuthService)
	permissionHandler := handler.NewAuditHandler(deps.PermissionService)

	serverRegisterHandler := handler.NewRegisterHandler(deps.ServerService)
	serverQueryHandler := handler.NewServerQueryHandler(deps.ServerService)
	serverStatusHandler := handler.NewServerStatusHandler(deps.ServerService)
	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)
	toolPermissionHandler := handler.NewPermissionHandler(deps.PermissionService)

	// 角色限流配置：每分钟最大请求数（QPS × 60）
	rateLimitCfg := map[string]int{
		"admin":     600, // 10 QPS
		"developer": 300, // 5 QPS
		"agent":     120, // 2 QPS
	}

	api := r.Group("/api")
	servers := api.Group("/servers")
	servers.POST("", serverRegisterHandler.RegisterServer)
	servers.GET("", serverQueryHandler.ListServers)
	servers.GET("/:id", serverQueryHandler.GetServer)
	servers.POST("/:id/activate", serverStatusHandler.ActivateServer)
	servers.POST("/:id/offline", serverStatusHandler.OfflineServer)
	servers.POST("/:id/health-check", serverStatusHandler.HealthCheck)

	tools := api.Group("/tools")
	tools.POST("", toolRegisterHandler.RegisterTool)
	tools.GET("", toolQueryHandler.ListTools)
	tools.GET("/:id", toolQueryHandler.GetTool)
	// 发布/下线/权限配置属于管理员工具审核入口，需要 admin 角色
	tools.POST("/:id/publish", middleware.Auth(deps.AuthService), middleware.RequireRole("admin"), toolPublishHandler.PublishTool)
	tools.POST("/:id/offline", middleware.Auth(deps.AuthService), middleware.RequireRole("admin"), toolPublishHandler.OfflineTool)
	tools.POST("/:id/permissions", middleware.Auth(deps.AuthService), middleware.RequireRole("admin"), toolPermissionHandler.SetPermission)

	// 认证接口（无需鉴权）
	api.POST("/auth/login", authHandler.Login)

	// 权限查询接口（兼容 permission.Client 的 HTTP 调用模式）
	api.GET("/permission/tools", permissionHandler.ListPermissions)

	// ===== 第3周：工具市场 =====
	marketRepo := repository.NewPostgresReviewRepository(deps.Pool)
	marketService := service.NewMarketService(toolRepo, serverRepo, marketRepo, deps.ClickHouse)
	marketHandler := handler.NewMarketHandler(marketService)
	openapiService := service.NewOpenAPIService(toolRepo, serverRepo)
	skillsService := service.NewSkillsService(toolRepo, serverRepo)
	integrationHandler := handler.NewIntegrationHandler(openapiService, skillsService)

	market := api.Group("/market", middleware.Auth(deps.AuthService))
	market.GET("/tools", marketHandler.ListTools)
	market.GET("/ranking", marketHandler.Ranking)
	market.GET("/tools/:id", marketHandler.GetTool)
	market.GET("/tools/:id/config-snippet", marketHandler.ConfigSnippet)
	market.GET("/tools/:id/reviews", marketHandler.ListReviews)
	market.POST("/tools/:id/reviews", marketHandler.UpsertReview)

	// OpenAPI / Skills 导入：管理员操作
	integration := api.Group("", middleware.Auth(deps.AuthService), middleware.RequireRole("admin"))
	integration.POST("/openapi/import", integrationHandler.ImportOpenAPI)
	integration.POST("/skills/import", integrationHandler.ImportSkills)

	// ===== 第4周：审计与统计 =====
	analyticsHandler := handler.NewAnalyticsHandler(deps.ClickHouse)
	// 统计概览：admin/developer 可查看
	stats := api.Group("", middleware.Auth(deps.AuthService), middleware.RequireRole("admin", "developer"))
	stats.GET("/analytics/overview", analyticsHandler.Overview)
	stats.GET("/audit/logs", analyticsHandler.AuditLogs)

	// 网关入口：工具发现 + 工具调用，需 JWT + RBAC + 限流
	mcp := r.Group("/mcp")
	mcp.Use(middleware.Auth(deps.AuthService), middleware.RateLimit(deps.Redis, rateLimitCfg))
	// 标准 MCP JSON-RPC 端点（Streamable HTTP 无状态）：initialize / tools/list / tools/call
	mcp.POST("", handler.NewMCPRPCHandler(proxyService).Handle)
	mcp.GET("/tools", proxyHandler.ListTools)
	mcp.POST("/tools/:toolName/call", proxyHandler.CallTool)
	return r
}
