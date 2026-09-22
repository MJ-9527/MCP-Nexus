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

// SetupRouter creates a Gin engine backed by PostgreSQL.
func SetupRouter(pool *pgxpool.Pool) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(gin.Logger())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())
	r.Use(middleware.Audit(nil))
	r.Use(middleware.RateLimit(60, 60*time.Second))

	r.GET("/health", handler.Health)

	userRepo := repository.NewPostgresUserRepository(pool)
	authSvc := service.NewUserService(userRepo)
	r.POST("/api/auth/login", handler.NewAuthHandler(authSvc).Login)
	r.POST("/api/auth/register", handler.NewAuthHandler(authSvc).Register)

	serverRepo := repository.NewPostgresServerRepository(pool)
	toolRepo := repository.NewPostgresToolRepository(pool)
	permRepo := repository.NewPostgresToolPermissionRepository(pool)
	mcpClient := client.NewMCPClient(5 * time.Second)

	serverSvc := service.NewServerService(serverRepo)
	toolSvc := service.NewToolService(toolRepo, serverRepo)
	permSvc := service.NewPermissionService(permRepo)
	gatewaySvc := service.NewGatewayService(toolRepo, serverRepo, mcpClient)
	healthSvc := service.NewServerHealthService(serverRepo, client.NewHealthClient(5*time.Second))

	analyticsSvc := service.NewAnalyticsService()
	alertsSvc := service.NewAlertService()

	api := r.Group("/api")
	{
		api.GET("/auth/me", middleware.OptionalAuth(), func(c *gin.Context) {
			uid, ok := middleware.GetCurrentUserID(c)
			if !ok {
				handler.RespondError(c, 401, "UNAUTHORIZED", "未认证")
				return
			}
			handler.RespondSuccess(c, gin.H{"user_id": uid, "username": middleware.GetCurrentUsername(c), "role": middleware.GetCurrentRole(c)})
		})

		servers := api.Group("/servers")
		{
			servers.POST("", middleware.AuthRequired(), handler.NewRegisterHandler(serverSvc).RegisterServer)
			servers.GET("", newServerQueryH(serverSvc).List)
			servers.GET("/:id", newServerQueryH(serverSvc).Get)
			servers.POST("/:id/activate", middleware.AuthRequired(), handler.NewServerStatusHandler(serverSvc).ActivateServer)
			servers.POST("/:id/offline", middleware.AuthRequired(), handler.NewServerStatusHandler(serverSvc).OfflineServer)
			servers.POST("/:id/health-check", middleware.AuthRequired(), handler.NewServerHealthHandler(healthSvc).CheckServer)
		}

		tools := api.Group("/tools")
		{
			tools.POST("", middleware.AuthRequired(), handler.NewToolRegisterHandler(toolSvc).RegisterTool)
			tools.GET("", newToolQueryH(toolSvc).List)
			tools.GET("/:id", newToolQueryH(toolSvc).Get)
			tools.POST("/:id/publish", middleware.AuthRequired(), handler.NewToolPublishHandler(toolSvc).PublishTool)
			tools.POST("/:id/offline", middleware.AuthRequired(), handler.NewToolPublishHandler(toolSvc).OfflineTool)
		}

		perms := api.Group("/permissions")
		{
			perms.POST("", middleware.AuthRequired(), handler.NewPermissionHandler(permSvc).Grant)
			perms.DELETE("/:toolId", middleware.AuthRequired(), handler.NewPermissionHandler(permSvc).Revoke)
			perms.GET("/:toolId", handler.NewPermissionHandler(permSvc).ListByTool)
			perms.GET("/:toolId/check", middleware.AuthRequired(), handler.NewPermissionHandler(permSvc).CheckPermission)
		}

		api.GET("/audit/logs", middleware.AuthRequired(), func(c *gin.Context) {
			handler.RespondSuccess(c, gin.H{"items": middleware.GetAuditEntries(), "total": len(middleware.GetAuditEntries())})
		})
		api.GET("/analytics/overview", middleware.AuthRequired(), handler.NewAnalyticsHandler(analyticsSvc).Overview)
		alertsGrp := api.Group("/alerts")
		{
			alertsGrp.GET("", middleware.AuthRequired(), handler.NewAlertsHandler(alertsSvc).List)
			alertsGrp.PUT("/:alertId/ack", middleware.AuthRequired(), handler.NewAlertsHandler(alertsSvc).Acknowledge)
		}
	}

	mcp := r.Group("/mcp")
	{
		mcp.GET("/tools", handler.NewMCPGatewayHandler(gatewaySvc).ListTools)
		mcp.POST("/tools/:toolName/call", handler.NewMCPGatewayHandler(gatewaySvc).CallTool)
	}
	return r
}

// SetupRouterFallback creates a Gin engine backed by in-memory repositories.
func SetupRouterFallback() *gin.Engine {
	memServer := repository.NewMemoryServerRepository()
	memTool := &repository.MemToolRepo{}
	memPerm := &repository.MemPermRepo{}
	memUser := &repository.MemUserRepo{}

	serverSvc := service.NewServerService(memServer)
	toolSvc := service.NewToolService(memTool, memServer)
	permSvc := service.NewPermissionService(memPerm)
	authSvc := service.NewUserService(memUser)
	mcpClient := client.NewMCPClient(5 * time.Second)
	gatewaySvc := service.NewGatewayService(memTool, memServer, mcpClient)
	healthSvc := service.NewServerHealthService(memServer, client.NewHealthClient(5*time.Second))

	analyticsSvc := service.NewAnalyticsService()
	alertsSvc := service.NewAlertService()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(gin.Logger())
	r.Use(middleware.CORS())
	r.Use(middleware.RequestID())
	r.Use(middleware.Audit(nil))
	r.Use(middleware.RateLimit(60, 60*time.Second))
	r.GET("/health", handler.Health)
	r.POST("/api/auth/login", handler.NewAuthHandler(authSvc).Login)
	r.POST("/api/auth/register", handler.NewAuthHandler(authSvc).Register)

	api := r.Group("/api")
	{
		api.GET("/auth/me", middleware.OptionalAuth(), func(c *gin.Context) {
			uid, ok := middleware.GetCurrentUserID(c)
			if !ok {
				handler.RespondError(c, 401, "UNAUTHORIZED", "未认证")
				return
			}
			handler.RespondSuccess(c, gin.H{"user_id": uid})
		})

		servers := api.Group("/servers")
		{
			servers.POST("", middleware.AuthRequired(), handler.NewRegisterHandler(serverSvc).RegisterServer)
			servers.GET("", newServerQueryH(serverSvc).List)
			servers.GET("/:id", newServerQueryH(serverSvc).Get)
			servers.POST("/:id/activate", middleware.AuthRequired(), handler.NewServerStatusHandler(serverSvc).ActivateServer)
			servers.POST("/:id/offline", middleware.AuthRequired(), handler.NewServerStatusHandler(serverSvc).OfflineServer)
			servers.POST("/:id/health-check", middleware.AuthRequired(), handler.NewServerHealthHandler(healthSvc).CheckServer)
		}

		tools := api.Group("/tools")
		{
			tools.POST("", middleware.AuthRequired(), handler.NewToolRegisterHandler(toolSvc).RegisterTool)
			tools.GET("", newToolQueryH(toolSvc).List)
			tools.GET("/:id", newToolQueryH(toolSvc).Get)
			tools.POST("/:id/publish", middleware.AuthRequired(), handler.NewToolPublishHandler(toolSvc).PublishTool)
			tools.POST("/:id/offline", middleware.AuthRequired(), handler.NewToolPublishHandler(toolSvc).OfflineTool)
		}

		perms := api.Group("/permissions")
		{
			perms.POST("", middleware.AuthRequired(), handler.NewPermissionHandler(permSvc).Grant)
		}

		api.GET("/audit/logs", middleware.AuthRequired(), func(c *gin.Context) {
			handler.RespondSuccess(c, gin.H{"items": middleware.GetAuditEntries(), "total": len(middleware.GetAuditEntries())})
		})
		api.GET("/analytics/overview", middleware.AuthRequired(), handler.NewAnalyticsHandler(analyticsSvc).Overview)
		alertsGrp := api.Group("/alerts")
		{
			alertsGrp.GET("", middleware.AuthRequired(), handler.NewAlertsHandler(alertsSvc).List)
			alertsGrp.PUT("/:alertId/ack", middleware.AuthRequired(), handler.NewAlertsHandler(alertsSvc).Acknowledge)
		}
	}

	mcp := r.Group("/mcp")
	{
		mcp.GET("/tools", handler.NewMCPGatewayHandler(gatewaySvc).ListTools)
		mcp.POST("/tools/:toolName/call", handler.NewMCPGatewayHandler(gatewaySvc).CallTool)
	}
	return r
}

type srvQ struct{ s *service.ServerService }
func newServerQueryH(s *service.ServerService) *srvQ { return &srvQ{s} }
func (w *srvQ) List(c *gin.Context) { handler.NewServerQueryHandler(w.s).ListServers(c) }
func (w *srvQ) Get(c *gin.Context)  { handler.NewServerQueryHandler(w.s).GetServer(c) }

type toolQ struct{ s *service.ToolService }
func newToolQueryH(s *service.ToolService) *toolQ { return &toolQ{s} }
func (w *toolQ) List(c *gin.Context) { handler.NewToolQueryHandler(w.s).ListTools(c) }
func (w *toolQ) Get(c *gin.Context)  { handler.NewToolQueryHandler(w.s).GetTool(c) }