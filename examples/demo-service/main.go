package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// demo-service 是用于联调的示例 MCP Server：
//
//	GET  /health                    —— 健康检查
//	POST /tools/query_sales/call    —— 返回脱敏销售数据（MCP 协议响应格式）
//
// 启动：go run ./examples/demo-service
func main() {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/tools/query_sales/call", func(c *gin.Context) {
		// 固定返回脱敏销售数据，遵循 MCP tools/call 响应协议：
		// {"content":[{"type":"text","text":"..."}],"is_error":false}
		c.JSON(http.StatusOK, gin.H{
			"content": []gin.H{
				{"type": "text", "text": `{"month":"2026-08","total_sales":128000,"region":"华东"}`},
			},
			"is_error": false,
		})
	})

	_ = r.Run(":9001")
}
