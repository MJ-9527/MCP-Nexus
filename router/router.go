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

	serverRepo := repository.NewPostgresServerRepository(pool)
	serverService := service.NewServerService(serverRepo)
	serverHealthService := service.NewServerHealthService(serverRepo, client.NewHealthClient(5*time.Second))
	toolRepo := repository.NewPostgresToolRepository(pool)
	toolService := service.NewToolService(toolRepo, serverRepo)

	serverRegisterHandler := handler.NewRegisterHandler(serverService)
	serverQueryHandler := handler.NewServerQueryHandler(serverService)
	serverStatusHandler := handler.NewServerStatusHandler(serverService)
	serverHealthHandler := handler.NewServerHealthHandler(serverHealthService)
	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)

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
	return r
}
