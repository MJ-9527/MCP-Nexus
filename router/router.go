package router

import (
	"net/http"
	"os"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/model"
	"MCP-Nexus/pkg/logutil"
	"MCP-Nexus/pkg/metrics"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupDemoRouter 创建 demo-service 的完整路由引擎。
// 负责 logger/metrics 初始化、中间件挂载、路由注册。
func SetupDemoRouter(db *pgxpool.Pool, fileBase ...string) *gin.Engine {
	logger := logutil.SetupLogger(os.Stderr)
	m := metrics.New()

	base := model.DefaultFileBase
	if len(fileBase) > 0 && fileBase[0] != "" {
		base = fileBase[0]
	}

	r := gin.New()
	r.Use(middleware.DemoRequestID())
	r.Use(middleware.DemoLogger(logger, m))
	r.Use(gin.Recovery())

	demoHandlers := handler.NewDemoHandlers(logger, m, db, base)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "demo-service",
			"time":    time.Now().Format(time.RFC3339),
			"health":  true,
		})
	})
	r.POST("/tools/query_sales/call", demoHandlers.QuerySales)
	r.POST("/tools/query_customer/call", demoHandlers.QueryCustomer)
	r.POST("/tools/read_file/call", demoHandlers.ReadFile)
	r.POST("/tools/fetch_url/call", demoHandlers.FetchURL)
	r.POST("/tools/delete_customer/call", middleware.DemoAPIKeyAuth(logger, m), demoHandlers.DeleteCustomer)
	r.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, m)
	})

	return r
}

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

	// 健康检查：探测 + 后台定时检查；持久化走 ServerRepository.UpdateHealth。
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
	servers.POST("/:serversId/activate", serverStatusHandler.ActivateServer)
	servers.POST("/:serversId/offline", serverStatusHandler.OfflineServer)
	servers.POST("/:serversId/health-check", healthCheckHandler.HealthCheck)

	tools := api.Group("/tools")
	tools.POST("", toolRegisterHandler.RegisterTool)
	tools.GET("", toolQueryHandler.ListTools)
	tools.GET("/:toolId", toolQueryHandler.GetTool)
	tools.POST("/:toolId/publish", toolPublishHandler.PublishTool)
	tools.POST("/:toolId/offline", toolPublishHandler.OfflineTool)

	return r, healthCheckService
}
