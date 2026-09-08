package router

import (
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
	registerHandler := handler.NewRegisterHandler(serverService)
	queryHandler := handler.NewServerQueryHandler(serverService)
	r.POST("/api/servers", registerHandler.RegisterServer)

	r.GET("/api/servers", queryHandler.ListServers)
	r.GET("/api/servers/:id", queryHandler.GetServer)
	return r
}
