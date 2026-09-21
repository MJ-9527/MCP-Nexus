package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Customer 是脱敏后的客户数据（数据库类示例工具）。
type Customer struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Email  string `json:"email"`
	Region string `json:"region"`
}

const (
	// defaultFileBase 是 read_file 工具默认读取的目录（容器内）。
	defaultFileBase = "/app/data"
	// maxFetchBodyBytes 限制外部 HTTP 响应体大小（1MB）。
	maxFetchBodyBytes = int64(1 << 20)
)

var (
	// allowedFetchHosts 是 fetch_url 工具的域名白名单，阻止任意 SSRF。
	allowedFetchHosts = map[string]bool{
		"example.com":      true,
		"postman-echo.com": true,
		"httpbin.org":      true,
	}

	// serviceAPIKey 是敏感工具的 API Key（可通过 API_KEY 环境变量覆盖）。
	// 敏感工具（如 delete_customer）必须携带正确的 X-API-Key Header 才会执行下游操作。
	serviceAPIKey = "demo-api-key"

	logger  *slog.Logger
	metrics *serviceMetrics
)

// serviceMetrics 是 demo-service 在内存中的基础指标。
type serviceMetrics struct {
	RequestsTotal      int64            `json:"requests_total"`
	RequestsByStatus   map[int]int64    `json:"requests_by_status"`
	ToolCallsTotal     int64            `json:"tool_calls_total"`
	ToolCallsByName    map[string]int64 `json:"tool_calls_by_name"`
	ErrorsTotal        int64            `json:"errors_total"`
	ErrorDetails       map[string]int64 `json:"error_details"`
}

func newServiceMetrics() *serviceMetrics {
	return &serviceMetrics{
		RequestsByStatus: make(map[int]int64),
		ToolCallsByName:  make(map[string]int64),
		ErrorDetails:     make(map[string]int64),
	}
}

func (m *serviceMetrics) recordRequest(status int) {
	m.RequestsTotal++
	m.RequestsByStatus[status]++
	if status >= 400 {
		m.ErrorsTotal++
	}
}

func (m *serviceMetrics) recordTool(tool string) {
	m.ToolCallsTotal++
	m.ToolCallsByName[tool]++
}

func (m *serviceMetrics) recordError(label string) {
	m.ErrorsTotal++
	m.ErrorDetails[label]++
}

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

	r.POST("/tools/query_sales/call", func(c *gin.Context) {
		start := timeNowMS()
		var payload map[string]any

		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			if err := c.ShouldBindJSON(&payload); err != nil {
				metrics.recordError("query_sales_invalid_json")
				logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "query_sales invalid json", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
				return
			}
		}

		metrics.recordTool("query_sales")
		logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logAttr(c.Request.Context(),
			slog.String("tool", "query_sales"),
			slog.Int64("duration_ms", timeNowMS()-start),
		)...)

		c.JSON(http.StatusOK, gin.H{
			"month":       "2026-08",
			"total_sales": 128000,
			"region":      "华东",
			"tool":        "query_sales",
			"request":     payload,
		})
	})

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

// requestIDMiddleware 为每个请求注入/读取 request_id，并放入 context。
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = uuid.New().String()
		}
		c.Set(requestIDKey{}, rid)
		c.Header("X-Request-ID", rid)
		c.Request = c.Request.WithContext(withRequestID(c.Request.Context(), rid))
		c.Next()
	}
}

// structuredLogger 记录每个请求的结构化访问日志（含 request_id、状态码、耗时、响应大小）。
func structuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rw := newResponseWriter(c.Writer)
		c.Writer = rw
		c.Next()
		metrics.recordRequest(rw.statusCode)

		logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "http request", logAttr(c.Request.Context(),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", rw.statusCode),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int64("response_size", rw.size),
		)...)
	}
}

// queryCustomer 从 PostgreSQL 查询脱敏客户数据。
// 错误场景：非法 JSON → 400；未配置数据库 → 503；查询失败 → 500。
func queryCustomer(c *gin.Context, db *pgxpool.Pool) {
	start := timeNowMS()
	var payload struct {
		Region string `json:"region"`
		Limit  int    `json:"limit"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			metrics.recordError("query_customer_invalid_json")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "query_customer invalid json", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.Limit <= 0 {
		payload.Limit = 20
	}
	if db == nil {
		metrics.recordError("query_customer_db_unavailable")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "query_customer database unavailable")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	rows, err := db.Query(ctx,
		`SELECT id, name, phone, email, region FROM demo_customers
		 WHERE ($1 = '' OR region = $1) ORDER BY id LIMIT $2`,
		payload.Region, payload.Limit)
	if err != nil {
		metrics.recordError("query_customer_query_failed")
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "query_customer query failed", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		var cu Customer
		if err := rows.Scan(&cu.ID, &cu.Name, &cu.Phone, &cu.Email, &cu.Region); err != nil {
			metrics.recordError("query_customer_scan_failed")
			logger.LogAttrs(c.Request.Context(), slog.LevelError, "query_customer scan failed", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}
		customers = append(customers, cu)
	}
	if err := rows.Err(); err != nil {
		metrics.recordError("query_customer_rows_error")
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "query_customer rows error", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	metrics.recordTool("query_customer")
	logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logAttr(c.Request.Context(),
		slog.String("tool", "query_customer"),
		slog.Int("count", len(customers)),
		slog.Int64("duration_ms", timeNowMS()-start),
	)...)

	c.JSON(http.StatusOK, gin.H{
		"tool":      "query_customer",
		"count":     len(customers),
		"customers": customers,
	})
}

// readFile 读取 baseDir 下的文件。
// 安全限制：拒绝路径穿越与越界访问，仅允许访问 baseDir 内部文件。
// 错误场景：非法 JSON → 400；缺 path → 400；路径穿越/越界 → 403；文件不存在 → 404；读取失败 → 500。
func readFile(c *gin.Context, baseDir string) {
	start := timeNowMS()
	var payload struct {
		Path string `json:"path"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			metrics.recordError("read_file_invalid_json")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file invalid json", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.Path == "" {
		metrics.recordError("read_file_missing_path")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file missing path")
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		metrics.recordError("read_file_base_dir_unavailable")
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "read_file base dir unavailable", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "base dir unavailable"})
		return
	}

	// 显式拒绝路径穿越意图。
	if strings.Contains(payload.Path, "..") {
		metrics.recordError("read_file_path_traversal")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file path traversal blocked", logAttr(c.Request.Context(), slog.String("path", payload.Path))...)
		c.JSON(http.StatusForbidden, gin.H{"error": "path traversal blocked"})
		return
	}

	target := filepath.Join(baseAbs, filepath.Clean(payload.Path))

	// 双保险：确保最终路径仍在 base 目录内。
	rel, err := filepath.Rel(baseAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		metrics.recordError("read_file_path_outside_base")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file path outside base dir", logAttr(c.Request.Context(), slog.String("path", payload.Path))...)
		c.JSON(http.StatusForbidden, gin.H{"error": "path outside base dir"})
		return
	}

	data, err := os.ReadFile(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			metrics.recordError("read_file_not_found")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file not found", logAttr(c.Request.Context(), slog.String("path", payload.Path))...)
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		metrics.recordError("read_file_failed")
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "read_file failed", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read failed"})
		return
	}

	metrics.recordTool("read_file")
	logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logAttr(c.Request.Context(),
		slog.String("tool", "read_file"),
		slog.String("path", payload.Path),
		slog.Int64("duration_ms", timeNowMS()-start),
	)...)

	c.JSON(http.StatusOK, gin.H{
		"tool":    "read_file",
		"path":    payload.Path,
		"content": string(data),
	})
}

// fetchURL 请求外部 URL，并返回经过脱敏的响应。
// 安全限制：域名白名单、SSRF（内网 IP）拦截、响应大小上限、敏感字段脱敏。
func fetchURL(c *gin.Context) {
	start := timeNowMS()
	var payload struct {
		URL string `json:"url"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			metrics.recordError("fetch_url_invalid_json")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url invalid json", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.URL == "" {
		metrics.recordError("fetch_url_missing_url")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url missing url")
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	u, err := url.Parse(payload.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		metrics.recordError("fetch_url_invalid_url")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url invalid url", logAttr(c.Request.Context(), slog.String("url", payload.URL))...)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		metrics.recordError("fetch_url_scheme_not_allowed")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url scheme not allowed", logAttr(c.Request.Context(), slog.String("scheme", u.Scheme))...)
		c.JSON(http.StatusForbidden, gin.H{"error": "scheme not allowed"})
		return
	}

	host := u.Hostname()
	if !allowedFetchHosts[host] {
		metrics.recordError("fetch_url_domain_not_allowed")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url domain not allowed", logAttr(c.Request.Context(), slog.String("host", host))...)
		c.JSON(http.StatusForbidden, gin.H{"error": "domain not allowed"})
		return
	}

	// SSRF 防护：解析域名并拒绝内网地址。
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		metrics.recordError("fetch_url_dns_failed")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url dns failed", logAttr(c.Request.Context(), slog.String("host", host), slog.String("error", err.Error()))...)
		c.JSON(http.StatusBadGateway, gin.H{"error": "dns resolve failed"})
		return
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			metrics.recordError("fetch_url_private_address")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url private address blocked", logAttr(c.Request.Context(), slog.String("ip", ip.String()))...)
			c.JSON(http.StatusForbidden, gin.H{"error": "private address blocked"})
			return
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(payload.URL)
	if err != nil {
		metrics.recordError("fetch_url_request_failed")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url request failed", logAttr(c.Request.Context(), slog.String("url", payload.URL), slog.String("error", err.Error()))...)
		c.JSON(http.StatusBadGateway, gin.H{"error": "request failed"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBodyBytes+1))
	if err != nil {
		metrics.recordError("fetch_url_read_response_failed")
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "fetch_url read response failed", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		c.JSON(http.StatusBadGateway, gin.H{"error": "read response failed"})
		return
	}
	if int64(len(body)) > maxFetchBodyBytes {
		metrics.recordError("fetch_url_response_too_large")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url response too large", logAttr(c.Request.Context(), slog.String("url", payload.URL), slog.Int("size", len(body)))...)
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "response too large"})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	metrics.recordTool("fetch_url")
	logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logAttr(c.Request.Context(),
		slog.String("tool", "fetch_url"),
		slog.String("url", payload.URL),
		slog.Int("status", resp.StatusCode),
		slog.Int64("duration_ms", timeNowMS()-start),
	)...)

	c.JSON(http.StatusOK, gin.H{
		"tool":         "fetch_url",
		"url":          payload.URL,
		"status":       resp.StatusCode,
		"content_type": contentType,
		"body":         redactSensitive(body, contentType),
	})
}

// isPrivateIP 判断 IP 是否属于内网/回环/链路本地/未指定地址。
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// redactSensitive 对 JSON 响应中的敏感字段做脱敏；非 JSON 返回原始文本。
func redactSensitive(data []byte, contentType string) any {
	if strings.Contains(contentType, "application/json") {
		var v any
		if err := json.Unmarshal(data, &v); err == nil {
			return redactValue(v)
		}
	}
	return string(data)
}

// redactValue 递归脱敏 map/slice 中的敏感键。
func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSensitiveKey(k) {
				out[k] = "***"
				continue
			}
			out[k] = redactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactValue(val)
		}
		return out
	default:
		return v
	}
}

// isSensitiveKey 判断键名是否包含敏感字段关键词。
func isSensitiveKey(k string) bool {
	lower := strings.ToLower(k)
	for _, s := range []string{"password", "passwd", "token", "secret", "api_key", "apikey", "authorization", "cookie"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// apiKeyAuth 是敏感工具的鉴权中间件。
// 校验 X-API-Key Header；缺失或不匹配时返回 401 并中止，不会进入下游处理器。
func apiKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key != serviceAPIKey {
			metrics.recordError("delete_customer_unauthorized")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer unauthorized", logAttr(c.Request.Context(), slog.Bool("has_key", key != ""))...)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// deleteCustomer 删除指定客户（敏感工具，需 API Key）。
// 鉴权由 apiKeyAuth 中间件处理；未通过时不会进入本函数，下游 DELETE 不会执行。
// 错误场景：非法 JSON → 400；缺 id → 400；未配置数据库 → 503；超时 → 504；删除失败 → 500。
func deleteCustomer(c *gin.Context, db *pgxpool.Pool) {
	start := timeNowMS()
	var payload struct {
		ID int64 `json:"id"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			metrics.recordError("delete_customer_invalid_json")
			logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer invalid json", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.ID <= 0 {
		metrics.recordError("delete_customer_missing_id")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer missing id")
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if db == nil {
		metrics.recordError("delete_customer_db_unavailable")
		logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer database unavailable")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	tag, err := db.Exec(ctx, `DELETE FROM demo_customers WHERE id = $1`, payload.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			metrics.recordError("delete_customer_timeout")
			logger.LogAttrs(c.Request.Context(), slog.LevelError, "delete_customer timeout", logAttr(c.Request.Context(), slog.Int64("id", payload.ID))...)
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "operation timed out"})
			return
		}
		metrics.recordError("delete_customer_failed")
		logger.LogAttrs(c.Request.Context(), slog.LevelError, "delete_customer failed", logAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}

	metrics.recordTool("delete_customer")
	logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logAttr(c.Request.Context(),
		slog.String("tool", "delete_customer"),
		slog.Int64("id", payload.ID),
		slog.Bool("deleted", tag.RowsAffected() > 0),
		slog.Int64("duration_ms", timeNowMS()-start),
	)...)

	c.JSON(http.StatusOK, gin.H{
		"tool":    "delete_customer",
		"id":      payload.ID,
		"deleted": tag.RowsAffected() > 0,
	})
}

func main() {
	logger = setupLogger(os.Stderr)
	metrics = newServiceMetrics()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// 可选：覆盖敏感工具的 API Key。
	if k := os.Getenv("API_KEY"); k != "" {
		serviceAPIKey = k
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
	if err := setupRouter(db, fileBase).Run(":" + port); err != nil {
		logger.Error("server exited", slog.String("error", err.Error()))
		panic(err)
	}
}
