package router

import (
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.RequestID())

	r.GET("/health", handler.Health)

	serverRepo := repository.NewMemoryServerRepository()
	serverService := service.NewServerService(serverRepo)
	registerHandler := handler.NewRegisterHandler(serverService)
	queryHandler := handler.NewServerQueryHandler(serverService)
	r.POST("/api/servers", registerHandler.RegisterServer)

	r.GET("/api/servers", queryHandler.ListServers)
	r.GET("/api/servers/:id", queryHandler.GetServer)

	// 健康检查（成员 C）：暂用内存 repo + HTTP client；持久化等成员 A 的 UpdateHealthStatus
	healthClient := client.NewHealthClient(5 * time.Second)
	healthCheckService := service.NewHealthCheckService(serverRepo, healthClient)
	healthCheckHandler := handler.NewHealthCheckHandler(healthCheckService)
	r.POST("/api/servers/:id/health-check", healthCheckHandler.HealthCheck)

	return r
}
