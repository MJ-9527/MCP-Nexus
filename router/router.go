package router

import (
	"net/http"
	"os"
	"log"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/config"
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/model"
	"MCP-Nexus/pkg/logutil"
	"MCP-Nexus/pkg/metrics"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

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

func SetupRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
	r, _ := SetupRouterWithAnalytics(pool, cfg, nil)
	return r
}

// SetupRouterWithAnalytics 装配 HTTP 引擎和后台审计写入器。
// 调用方必须在进程退出前调用返回 writer 的 Stop，以刷新队列中的剩余日志。
func SetupRouterWithAnalytics(pool *pgxpool.Pool, cfg config.Config, clickhouseConn clickhouse.Conn) (*gin.Engine, *service.BatchAuditWriter) {
	r := gin.Default()
	r.Use(middleware.RequestID())

	r.GET("/health", handler.Health)

	// Repository → Service → Handler 装配（网关不绕过 Repository，不直接操作数据库）
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
	userRepo := repository.NewPostgresUserRepository(pool)
	permissionRepo := repository.NewPostgresToolPermissionRepository(pool)
	permissionService := service.NewToolPermissionService(permissionRepo, toolRepo, userRepo)
	ratingRepo := repository.NewPostgresToolRatingRepository(pool)
	ratingService := service.NewToolRatingService(ratingRepo, toolRepo)
	adaptationRepo := repository.NewPostgresAdaptationTaskRepository(pool)
	adaptationService := service.NewAdaptationService(adaptationRepo, toolRepo)
	auditRepo := repository.NewPostgresAuditLogRepository(pool)
	auditService := service.NewAuditLogService(auditRepo)
	// B14：审计异步批量写入器（500ms 或 100 条触发 flush）。由调用方（main.go）
	// 负责在启动时 Start、退出前 Stop。注入到 auditService 后 Submit 走异步入队。
	auditWriter := service.NewBatchAuditWriter(auditRepo, service.DefaultBatchSize, service.DefaultBatchInterval, service.DefaultQueueCapacity)
	// B14：ClickHouse 审计分析存储（可选）。配置了 CLICKHOUSE_HTTP_ADDR 才启用；
	// 启用后同一批审计记录会额外投递到 ClickHouse，PostgreSQL 主存储与查询接口不受影响。
	// 同时把 /api/metrics 的指标来源切到 ClickHouse 聚合（可回溯、重启不丢）。
	var auditMetrics repository.AuditMetricsRepository
	if cfg.ClickHouseAddr != "" {
		sink, err := repository.NewClickHouseAuditSink(
			cfg.ClickHouseAddr, cfg.ClickHouseDatabase, repository.DefaultClickHouseTable,
			cfg.ClickHouseUser, cfg.ClickHousePassword, 5*time.Second)
		if err != nil {
			log.Printf("[audit] ClickHouse 分析存储配置无效，本次仅写 PostgreSQL：%v", err)
		} else {
			auditWriter.SetSink(sink)
			log.Printf("[audit] 已启用 ClickHouse 分析存储 %s(%s.%s)", cfg.ClickHouseAddr, cfg.ClickHouseDatabase, repository.DefaultClickHouseTable)
		}
		// 聚合查询比单次写入慢，超时放宽到 10s
		metricsRepo, err := repository.NewClickHouseMetricsRepository(
			cfg.ClickHouseAddr, cfg.ClickHouseDatabase, repository.DefaultClickHouseTable,
			cfg.ClickHouseUser, cfg.ClickHousePassword, 10*time.Second)
		if err != nil {
			log.Printf("[metrics] ClickHouse 指标源配置无效，/api/metrics 将使用进程内指标：%v", err)
		} else {
			auditMetrics = metricsRepo
			log.Printf("[metrics] /api/metrics 指标来源：ClickHouse 聚合")
		}
	}
	auditService.SetBatchWriter(auditWriter)
	// B10：OpenAPI 翻译元数据 repository + 导入器
	specRepo := repository.NewPostgresOpenAPISpecRepository(pool)
	// B11：Skills 调用元数据 repository + 导入器
	skillsSpecRepo := repository.NewPostgresSkillsSpecRepository(pool)
	alertRepo := repository.NewPostgresAlertRepository(pool)
	var analyticsService *service.AnalyticsService
	if clickhouseConn != nil {
		analyticsService = service.NewAnalyticsService(repository.NewClickHouseAnalyticsRepository(clickhouseConn), alertRepo)
	}

	serverRegisterHandler := handler.NewRegisterHandler(serverService)
	serverQueryHandler := handler.NewServerQueryHandler(serverService)
	serverStatusHandler := handler.NewServerStatusHandler(serverService)

	// 健康检查：探测 + 后台定时检查；持久化走 ServerRepository.UpdateHealth。
	healthCheckService := service.NewHealthCheckService(serverRepo, healthClient)
	healthCheckHandler := handler.NewHealthCheckHandler(healthCheckService)

	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)
	permissionHandler := handler.NewToolPermissionHandler(permissionService)
	ratingHandler := handler.NewToolRatingHandler(ratingService)
	adaptationHandler := handler.NewAdaptationHandler(adaptationService)
	auditHandler := handler.NewAuditLogHandler(auditService)
	var analyticsHandler *handler.AnalyticsHandler
	if analyticsService != nil {
		analyticsHandler = handler.NewAnalyticsHandler(analyticsService)
	}
	authService := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTTTL)
	authHandler := handler.NewAuthHandler(authService)
	// B10：OpenAPI 导入器 + handler
	openapiImporter := service.NewOpenAPIImporter(serverRepo, toolRepo, specRepo)
	openapiImportHandler := handler.NewOpenAPIImportHandler(openapiImporter)
	// B11：Skills 导入器 + handler
	skillsImporter := service.NewSkillsImporter(serverRepo, toolRepo, skillsSpecRepo)
	skillsImportHandler := handler.NewSkillsImportHandler(skillsImporter)
	// B14：指标采集器（handler 在 proxySvc 装配后创建）
	metricsCollector := service.NewMetricsCollector(service.DefaultMetricsBufferSize)

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
	serversManage.POST("/:id/import-openapi", openapiImportHandler.Import) // B10
	serversManage.POST("/:id/import-skills", skillsImportHandler.Import)   // B11

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
	if analyticsHandler != nil {
		analyticsRoutes := manage.Group("/analytics")
		analyticsRoutes.GET("/overview", analyticsHandler.Overview)
		analyticsRoutes.GET("/trends", analyticsHandler.Trends)
		analyticsRoutes.GET("/alerts", analyticsHandler.ListAlerts)
		analyticsRoutes.POST("/alerts/:id/acknowledge", analyticsHandler.Acknowledge)
	}

	// MCP 网关代理（B1）：工具发现 + 调用转发，经统一调用链（权限过滤 → 状态检查 → 转发）
	// B5：/mcp 强制 JWT 认证，角色与用户 ID 由令牌注入，不再信任 X-Role
	// B7：JWT 之后挂 Redis 令牌桶限流（platform_admin 600/tool_developer 300/agent_caller 120 每分钟），Redis 不可用降级放行
	permissionClient := service.NewRolePermissionClient(permissionRepo)
	proxySvc := service.NewProxyService(serverRepo, toolRepo, permissionClient)
	proxySvc.SetAudit(auditService)       // B8：调用链审计埋点（B14 起经 Submit 异步入队）
	proxySvc.SetMetrics(metricsCollector) // B14：调用链指标采集
	// B10：注入 OpenAPI 翻译依赖（specRepo + translator），调用链据此检测 OpenAPI 工具
	proxySvc.SetOpenAPIDeps(specRepo, service.NewOpenAPITranslator(client.NewMCPClient(service.CallTimeout, client.DefaultMaxBodyBytes)))
	// B11：注入 Skills 翻译依赖（skillsSpecRepo + translator），调用链据此检测 Skills 工具
	proxySvc.SetSkillsDeps(skillsSpecRepo, service.NewSkillsTranslator(client.NewMCPClient(service.CallTimeout, client.DefaultMaxBodyBytes)))
	proxyHandler := handler.NewProxyHandler(proxySvc)
	// B14：指标查询 handler（依赖已装配完毕的 proxySvc 与可选的 ClickHouse 聚合源）
	metricsHandler := handler.NewMetricsHandler(proxySvc, auditMetrics)
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

	// B14：可观测性统计接口（admin 鉴权）
	manage.GET("/metrics", metricsHandler.GetMetrics)

	auditWriter.Start()

	return r, auditWriter
}
