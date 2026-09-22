package main

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// setupRouter 注册 demo-service 的所有路由与中间件。
func setupRouter(db *pgxpool.Pool, fileBase ...string) *gin.Engine {
	// 测试直接调用 setupRouter 时初始化全局依赖。
	if logger == nil {
		logger = setupLogger(os.Stderr)
	}
	if metrics == nil {
		metrics = newServiceMetrics()
	}

	base := defaultFileBase
	if len(fileBase) > 0 && fileBase[0] != "" {
		base = fileBase[0]
	}

	r := gin.New()
	r.Use(requestIDMiddleware())
	r.Use(structuredLogger())
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "demo-service",
			"time":    time.Now().Format(time.RFC3339),
			"health":  true,
		})
	})

	r.POST("/tools/query_sales/call", handleQuerySales)

	// 数据库类示例工具，查询脱敏客户数据。
	r.POST("/tools/query_customer/call", func(c *gin.Context) {
		queryCustomer(c, db)
	})

	// 文件类示例工具：读取 base 目录内的文件。
	r.POST("/tools/read_file/call", func(c *gin.Context) {
		readFile(c, base)
	})

	// HTTP 类示例工具：请求白名单内的外部 URL。
	r.POST("/tools/fetch_url/call", func(c *gin.Context) {
		fetchURL(c)
	})

	// 敏感工具：删除客户数据，需要 API Key 鉴权（无权限时不会执行下游 DELETE）。
	r.POST("/tools/delete_customer/call", apiKeyAuth(), func(c *gin.Context) {
		deleteCustomer(c, db)
	})

	// 基础指标端点：返回内存中的请求/工具/错误统计（JSON 格式）。
	r.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, metrics)
	})

	return r
}
