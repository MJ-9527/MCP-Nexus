package router

import (
	"log"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/config"
	"MCP-Nexus/handler"
	"MCP-Nexus/middleware"
	"MCP-Nexus/repository"
	"MCP-Nexus/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func SetupRouter(pool *pgxpool.Pool, cfg config.Config) *gin.Engine {
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
	// B14：审计异步批量写入器（500ms 或 100 条触发 flush）。由调用方（main.go）
	// 负责在启动时 Start、退出前 Stop。注入到 auditService 后 Submit 走异步入队。
	auditWriter := service.NewBatchAuditWriter(auditRepo, service.DefaultBatchSize, service.DefaultBatchInterval, service.DefaultQueueCapacity)
	// B14：ClickHouse 审计分析存储（可选）。配置了 CLICKHOUSE_HTTP_ADDR 才启用；
	// 启用后同一批审计记录会额外投递到 ClickHouse，PostgreSQL 主存储与查询接口不受影响。
	// 同时把 /api/metrics 的指标来源切到 ClickHouse 聚合（可回溯、重启不丢）。
	var auditMetrics repository.AuditMetricsRepository
	if cfg.ClickHouseAddr != "" {
		sink, err := repository.NewClickHouseAuditSink(
			cfg.ClickHouseAddr, cfg.ClickHouseDB, repository.DefaultClickHouseTable,
			cfg.ClickHouseUser, cfg.ClickHousePass, 5*time.Second)
		if err != nil {
			log.Printf("[audit] ClickHouse 分析存储配置无效，本次仅写 PostgreSQL：%v", err)
		} else {
			auditWriter.SetSink(sink)
			log.Printf("[audit] 已启用 ClickHouse 分析存储 %s(%s.%s)", cfg.ClickHouseAddr, cfg.ClickHouseDB, repository.DefaultClickHouseTable)
		}
		// 聚合查询比单次写入慢，超时放宽到 10s
		metricsRepo, err := repository.NewClickHouseMetricsRepository(
			cfg.ClickHouseAddr, cfg.ClickHouseDB, repository.DefaultClickHouseTable,
			cfg.ClickHouseUser, cfg.ClickHousePass, 10*time.Second)
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

	serverRegisterHandler := handler.NewRegisterHandler(serverService)
	serverQueryHandler := handler.NewServerQueryHandler(serverService)
	serverStatusHandler := handler.NewServerStatusHandler(serverService)
	serverHealthHandler := handler.NewServerHealthHandler(serverHealthService)
	toolRegisterHandler := handler.NewToolRegisterHandler(toolService)
	toolQueryHandler := handler.NewToolQueryHandler(toolService)
	toolPublishHandler := handler.NewToolPublishHandler(toolService)
	permissionHandler := handler.NewToolPermissionHandler(permissionService)
	auditHandler := handler.NewAuditLogHandler(auditService)
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
	manage.Use(middleware.RequireRole("admin"))
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
	toolsManage := manage.Group("/tools")
	toolsManage.POST("", toolRegisterHandler.RegisterTool)
	toolsManage.POST("/:id/publish", toolPublishHandler.PublishTool)
	toolsManage.POST("/:id/offline", toolPublishHandler.OfflineTool)

	permissions := toolsManage.Group("/:id/permissions")
	permissions.POST("", permissionHandler.GrantPermission)
	permissions.DELETE("", permissionHandler.RevokePermission)
	permissions.GET("", permissionHandler.ListPermissions)
	permissions.GET("/check", permissionHandler.CheckPermission)

	auditLogs := manage.Group("/audit-logs")
	auditLogs.POST("", auditHandler.Create)
	auditLogs.GET("", auditHandler.List)
	audit := api.Group("/audit/logs")
	audit.POST("", auditHandler.Create)
	audit.GET("", auditHandler.List)

	// MCP 网关代理（B1）：工具发现 + 调用转发，经统一调用链（权限过滤 → 状态检查 → 转发）
	// B5：/mcp 强制 JWT 认证，角色与用户 ID 由令牌注入，不再信任 X-Role
	// B7：JWT 之后挂 Redis 令牌桶限流（角色限额 admin 600/dev 300/agent 120 每分钟），Redis 不可用降级放行
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

	// B14：启动审计批量写入器后台 goroutine，并把 writer 暴露给包级变量，
	// 供 main.go 在收到退出信号时优雅 Stop（保证残留记录落地）。
	AuditWriterForShutdown = auditWriter
	auditWriter.Start()

	return r
}

// AuditWriterForShutdown 暴露 writer 引用供 main.go 在退出时 Stop。
// 当前简化方案：未来应改为 SetupRouter 返回包含 writer 的 ServiceBundle。
var AuditWriterForShutdown *service.BatchAuditWriter
