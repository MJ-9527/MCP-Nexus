package router

import (
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SetupRouter(pool *pgxpool.Pool) *gin.Engine {
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

	api := r.Group("/api")
	servers := api.Group("/servers")
	servers.POST("", serverRegisterHandler.RegisterServer)
	servers.GET("", serverQueryHandler.ListServers)
	servers.GET("/:id", serverQueryHandler.GetServer)
	servers.POST("/:id/activate", serverStatusHandler.ActivateServer)
	servers.POST("/:id/offline", serverStatusHandler.OfflineServer)
	servers.POST("/:id/health-check", serverHealthHandler.CheckServer)

	tools := api.Group("/tools")
	tools.POST("", toolRegisterHandler.RegisterTool)
	tools.GET("", toolQueryHandler.ListTools)
	tools.GET("/:id", toolQueryHandler.GetTool)
	tools.POST("/:id/publish", toolPublishHandler.PublishTool)
	tools.POST("/:id/offline", toolPublishHandler.OfflineTool)

	permissions := tools.Group("/:id/permissions")
	permissions.POST("", permissionHandler.GrantPermission)
	permissions.DELETE("", permissionHandler.RevokePermission)
	permissions.GET("", permissionHandler.ListPermissions)
	permissions.GET("/check", permissionHandler.CheckPermission)

	auditLogs := api.Group("/audit-logs")
	auditLogs.POST("", auditHandler.Create)
	auditLogs.GET("", auditHandler.List)

	// MCP 网关代理（B1）：工具发现 + 调用转发，经统一调用链（权限过滤 → 状态检查 → 转发）
	permissionClient := service.NewRolePermissionClient(permissionRepo)
	proxySvc := service.NewProxyService(serverRepo, toolRepo, permissionClient)
	proxyHandler := handler.NewProxyHandler(proxySvc)
	mcp := r.Group("/mcp")
	{
		mcp.GET("/tools", proxyHandler.ListTools)
		mcp.POST("/tools/:toolName/call", proxyHandler.CallTool)
	}

	return r
}
