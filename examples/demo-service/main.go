<<<<<<< HEAD
// Package main implements a demo MCP server for testing the gateway.
// Provides: /health, /tools/query_sales/call, /tools/list_products/call, /tools/delete_customer/call (auth-gated)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultPort = "9001"

// --- response helpers ---

func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, code, msg string) {
	jsonResponse(w, status, map[string]any{"code": code, "message": msg, "timestamp": time.Now().UTC().Format(time.RFC3339)})
}

// --- health ---

func healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]any{
		"status":    "healthy",
		"service":   "demo-mcp-server",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// --- query_sales ---

func querySalesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持 POST")
		return
	}
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "INVALID_PARAMETER", "请求体解析失败")
		return
	}
	month, _ := req["month"].(string)
	region, _ := req["region"].(string)
	if month == "" {
		month = "2026-08"
	}
	if region == "" {
		region = "华东"
	}
	salesMap := map[string]float64{
		"2026-08": 128000, "2026-07": 115000, "2026-06": 132000,
		"2026-05": 121000, "2026-04": 109000, "2026-03": 145000,
	}
	totalSales := salesMap[month]
	if totalSales == 0 {
		totalSales = 120000
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"month":       month,
		"total_sales": totalSales,
		"region":      region,
		"source":      "脱敏业务数据库",
	})
}

// --- list_products ---

func listProductsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持 POST")
		return
	}
	products := []map[string]any{
		{"id": 1, "name": "云服务器 ECS", "category": "基础设施", "price": 1200.00, "stock": 50},
		{"id": 2, "name": "对象存储 OSS", "category": "存储", "price": 350.00, "stock": 100},
		{"id": 3, "name": "关系型数据库 RDS", "category": "数据库", "price": 800.00, "stock": 30},
		{"id": 4, "name": "消息队列 RocketMQ", "category": "中间件", "price": 600.00, "stock": 45},
		{"id": 5, "name": "内容分发 CDN", "category": "网络", "price": 400.00, "stock": 60},
	}
	jsonResponse(w, http.StatusOK, map[string]any{"products": products, "total": len(products)})
}

// --- delete_customer (sensitive tool, requires X-Demo-Auth header) ---

func deleteCustomerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "只支持 POST")
		return
	}
	auth := r.Header.Get("X-Demo-Auth")
	if auth != "demo-secret-token" {
		jsonError(w, http.StatusForbidden, "FORBIDDEN", "缺少敏感工具访问权限，请提供有效的认证 Header")
		return
	}
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "INVALID_PARAMETER", "请求体解析失败")
		return
	}
	customerID, _ := req["customer_id"].(float64)
	jsonResponse(w, http.StatusOK, map[string]any{
		"deleted":     true,
		"customer_id": fmt.Sprintf("%.0f", customerID),
		"note":        "此操作已记录审计日志（脱敏）",
	})
}

// --- router ---

func toolHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tools/")
	parts := strings.SplitN(path, "/", 2)
	toolName := parts[0]
	switch toolName {
	case "query_sales":
		querySalesHandler(w, r)
	case "list_products":
		listProductsHandler(w, r)
	case "delete_customer":
		deleteCustomerHandler(w, r)
	default:
		jsonError(w, http.StatusNotFound, "TOOL_NOT_FOUND", "工具不存在: "+toolName)
	}
}

func main() {
	port := os.Getenv("DEMO_SERVICE_PORT")
	if port == "" {
		port = defaultPort
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/tools/", toolHandler)

	addr := ":" + port
	log.Printf("Demo MCP Server starting on %s", addr)
	log.Printf("  GET  %s/health", addr)
	log.Printf("  POST %s/tools/query_sales/call", addr)
	log.Printf("  POST %s/tools/list_products/call", addr)
	log.Printf("  POST %s/tools/delete_customer/call  (需 X-Demo-Auth: demo-secret-token)", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
=======
// demo-service 模拟下游 MCP Server：提供 query_sales / query_inventory 两个工具。
// 网关契约：POST {endpoint}，请求体 {"method":"tools/call","toolName":"...","arguments":{...}}，
// 响应体 {"content":[{"type":"text","text":"..."}],"is_error":false}。
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"MCP-Nexus/pkg/logutil"
	"MCP-Nexus/pkg/security"
	"MCP-Nexus/router"

	"github.com/jackc/pgx/v5/pgxpool"
)

type callRequest struct {
	Method    string                 `json:"method"`
	ToolName  string                 `json:"toolName"`
	Arguments map[string]interface{} `json:"arguments"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callResponse struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"is_error,omitempty"`
}

func main() {
	logger := logutil.SetupLogger(os.Stderr)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// 可选：覆盖敏感工具的 API Key。
	if k := os.Getenv("API_KEY"); k != "" {
		security.ServiceAPIKey = k
	}

	// 可选：配置 DATABASE_URL 后启用数据库类工具。
	var db *pgxpool.Pool
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := pgxpool.New(ctx, dsn)
		cancel()
		if err != nil {
			logger.Error("connect database failed", slog.String("error", err.Error()))
			panic(err)
		}
		db = pool
		defer db.Close()
	}

	fileBase := os.Getenv("FILE_BASE_DIR")

	logger.Info("demo-service starting", slog.String("port", port))
	if err := router.SetupDemoRouter(db, fileBase).Run(":" + port); err != nil {
		logger.Error("server exited", slog.String("error", err.Error()))
		return
	}
>>>>>>> origin/pull-request
}
