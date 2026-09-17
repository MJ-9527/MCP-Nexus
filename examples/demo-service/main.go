package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// allowedFetchHosts 是 fetch_url 工具的域名白名单，阻止任意 SSRF。
var allowedFetchHosts = map[string]bool{
	"example.com":      true,
	"postman-echo.com": true,
	"httpbin.org":      true,
}

// serviceAPIKey 是敏感工具的 API Key（可通过 API_KEY 环境变量覆盖）。
// 敏感工具（如 delete_customer）必须携带正确的 X-API-Key Header 才会执行下游操作。
var serviceAPIKey = "demo-api-key"

func setupRouter(db *pgxpool.Pool, fileBase ...string) *gin.Engine {
	base := defaultFileBase
	if len(fileBase) > 0 && fileBase[0] != "" {
		base = fileBase[0]
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "demo-service",
			"time":    time.Now().Format(time.RFC3339),
			"health":  true,
		})
	})

	r.POST("/tools/query_sales/call", func(c *gin.Context) {
		var payload map[string]any

		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			if err := c.ShouldBindJSON(&payload); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid json",
				})
				return
			}
		}

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

	return r
}

// queryCustomer 从 PostgreSQL 查询脱敏客户数据。
// 错误场景：非法 JSON → 400；未配置数据库 → 503；查询失败 → 500。
func queryCustomer(c *gin.Context, db *pgxpool.Pool) {
	var payload struct {
		Region string `json:"region"`
		Limit  int    `json:"limit"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.Limit <= 0 {
		payload.Limit = 20
	}
	if db == nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	customers := make([]Customer, 0)
	for rows.Next() {
		var cu Customer
		if err := rows.Scan(&cu.ID, &cu.Name, &cu.Phone, &cu.Email, &cu.Region); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}
		customers = append(customers, cu)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

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
	var payload struct {
		Path string `json:"path"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "base dir unavailable"})
		return
	}

	// 显式拒绝路径穿越意图。
	if strings.Contains(payload.Path, "..") {
		c.JSON(http.StatusForbidden, gin.H{"error": "path traversal blocked"})
		return
	}

	target := filepath.Join(baseAbs, filepath.Clean(payload.Path))

	// 双保险：确保最终路径仍在 base 目录内。
	rel, err := filepath.Rel(baseAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "path outside base dir"})
		return
	}

	data, err := os.ReadFile(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":    "read_file",
		"path":    payload.Path,
		"content": string(data),
	})
}

// fetchURL 请求外部 URL，并返回经过脱敏的响应。
// 安全限制：域名白名单、SSRF（内网 IP）拦截、响应大小上限、敏感字段脱敏。
func fetchURL(c *gin.Context) {
	var payload struct {
		URL string `json:"url"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	u, err := url.Parse(payload.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		c.JSON(http.StatusForbidden, gin.H{"error": "scheme not allowed"})
		return
	}

	host := u.Hostname()
	if !allowedFetchHosts[host] {
		c.JSON(http.StatusForbidden, gin.H{"error": "domain not allowed"})
		return
	}

	// SSRF 防护：解析域名并拒绝内网地址。
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "dns resolve failed"})
		return
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			c.JSON(http.StatusForbidden, gin.H{"error": "private address blocked"})
			return
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(payload.URL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "request failed"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "read response failed"})
		return
	}
	if int64(len(body)) > maxFetchBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "response too large"})
		return
	}

	contentType := resp.Header.Get("Content-Type")
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
		if c.GetHeader("X-API-Key") != serviceAPIKey {
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
	var payload struct {
		ID int64 `json:"id"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.ID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	tag, err := db.Exec(ctx, `DELETE FROM demo_customers WHERE id = $1`, payload.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "operation timed out"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":    "delete_customer",
		"id":      payload.ID,
		"deleted": tag.RowsAffected() > 0,
	})
}

func main() {
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
			panic(err)
		}
		db = pool
		defer db.Close()
	}

	fileBase := os.Getenv("FILE_BASE_DIR")

	if err := setupRouter(db, fileBase).Run(":" + port); err != nil {
		panic(err)
	}
}
