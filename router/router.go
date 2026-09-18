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

func SetupRouter(pool *pgxpool.Pool) (*gin.Engine, *service.HealthCheckService) {
	r := gin.Default()
	r.Use(middleware.RequestID())

	serverRepo := repository.NewPostgresServerRepository(pool)

	healthClient := client.NewHealthClient(5 * time.Second)
	healthHandler := handler.NewHealthHandler(pool, healthClient, []handler.DownstreamService{
		{Name: "demo-service", Endpoint: "http://demo-service:8081"},
		{Name: "skills-adapter", Endpoint: "http://demo-skills:8082"},
	})
	r.GET("/health", healthHandler.Health)

	serverService := service.NewServerService(serverRepo)
	toolRepo := repository.NewPostgresToolRepository(pool)
	toolService := service.NewToolService(toolRepo, serverRepo)

	serverRegisterHandler := handler.NewRegisterHandler(serverService)
	serverQueryHandler := handler.NewServerQueryHandler(serverService)
	serverStatusHandler := handler.NewServerStatusHandler(serverService)

	// 健康检查（成员 C）：探测 + 后台定时检查；持久化走 ServerRepository.UpdateHealth。
	healthCheckService := service.NewHealthCheckService(serverRepo, healthClient)
	healthCheckHandler := handler.NewHealthCheckHandler(healthCheckService)

	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)

	api := r.Group("/api")
	servers := api.Group("/servers")
	servers.POST("", serverRegisterHandler.RegisterServer)
	servers.GET("", serverQueryHandler.ListServers)
	servers.GET("/:serversId", serverQueryHandler.GetServer)
	servers.POST("/::serversId/activate", serverStatusHandler.ActivateServer)
	servers.POST("/::serversId/offline", serverStatusHandler.OfflineServer)
	servers.POST("/:id/health-check", healthCheckHandler.HealthCheck)

	tools := api.Group("/tools")
	tools.POST("", toolRegisterHandler.RegisterTool)
	tools.GET("", toolQueryHandler.ListTools)
	tools.GET("/:toolId", toolQueryHandler.GetTool)
	tools.POST("/:toolId/publish", toolPublishHandler.PublishTool)
	tools.POST("/:toolId/offline", toolPublishHandler.OfflineTool)

	return r, healthCheckService
}
