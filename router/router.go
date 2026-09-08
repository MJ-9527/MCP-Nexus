package router

import (
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

	//MCP网关代理
	toolRepo := repository.NewMemoryToolRepository()
	proxySvc := service.NewProxyService(serverRepo, toolRepo)
	proxyHandler := handler.NewProxyHandler(proxySvc)

	gatewayGroup := r.Group("/gateway")
	{
		gatewayGroup.POST("/tools/list", proxyHandler.ListTools)
		gatewayGroup.POST("/tools/call", proxyHandler.CallTool)
	}
	return r
}
