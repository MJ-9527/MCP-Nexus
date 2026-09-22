package handler

import (
	"context"
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

	"MCP-Nexus/model"
	"MCP-Nexus/pkg/logutil"
	"MCP-Nexus/pkg/metrics"
	"MCP-Nexus/pkg/security"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DemoHandlers 承载 demo-service 示例工具的处理逻辑。
type DemoHandlers struct {
	logger  *slog.Logger
	metrics *metrics.ServiceMetrics
	db      *pgxpool.Pool
	base    string
}

// NewDemoHandlers 创建 demo-service 工具处理器。
// db 可为 nil（未配置数据库时相关工具返回 503）；base 是 read_file 允许读取的根目录。
func NewDemoHandlers(logger *slog.Logger, m *metrics.ServiceMetrics, db *pgxpool.Pool, base string) *DemoHandlers {
	return &DemoHandlers{
		logger:  logger,
		metrics: m,
		db:      db,
		base:    base,
	}
}

// QuerySales 处理 query_sales 工具调用。
func (h *DemoHandlers) QuerySales(c *gin.Context) {
	start := logutil.TimeNowMS()
	var payload map[string]any

	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			if h.metrics != nil {
				h.metrics.RecordError("query_sales_invalid_json")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "query_sales invalid json", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}

	if h.metrics != nil {
		h.metrics.RecordTool("query_sales")
	}
	if h.logger != nil {
		h.logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logutil.LogAttr(c.Request.Context(),
			slog.String("tool", "query_sales"),
			slog.Int64("duration_ms", logutil.TimeNowMS()-start),
		)...)
	}

	c.JSON(http.StatusOK, gin.H{
		"month":       "2026-08",
		"total_sales": 128000,
		"region":      "华东",
		"tool":        "query_sales",
		"request":     payload,
	})
}

// QueryCustomer 从 PostgreSQL 查询脱敏客户数据。
// 错误场景：非法 JSON → 400；未配置数据库 → 503；查询失败 → 500。
func (h *DemoHandlers) QueryCustomer(c *gin.Context) {
	start := logutil.TimeNowMS()
	var payload struct {
		Region string `json:"region"`
		Limit  int    `json:"limit"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			if h.metrics != nil {
				h.metrics.RecordError("query_customer_invalid_json")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "query_customer invalid json", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.Limit <= 0 {
		payload.Limit = 20
	}
	if h.db == nil {
		if h.metrics != nil {
			h.metrics.RecordError("query_customer_db_unavailable")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "query_customer database unavailable")
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	rows, err := h.db.Query(ctx,
		`SELECT id, name, phone, email, region FROM demo_customers
		 WHERE ($1 = '' OR region = $1) ORDER BY id LIMIT $2`,
		payload.Region, payload.Limit)
	if err != nil {
		if h.metrics != nil {
			h.metrics.RecordError("query_customer_query_failed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "query_customer query failed", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	customers := make([]model.Customer, 0)
	for rows.Next() {
		var cu model.Customer
		if err := rows.Scan(&cu.ID, &cu.Name, &cu.Phone, &cu.Email, &cu.Region); err != nil {
			if h.metrics != nil {
				h.metrics.RecordError("query_customer_scan_failed")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "query_customer scan failed", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}
		customers = append(customers, cu)
	}
	if err := rows.Err(); err != nil {
		if h.metrics != nil {
			h.metrics.RecordError("query_customer_rows_error")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "query_customer rows error", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}

	if h.metrics != nil {
		h.metrics.RecordTool("query_customer")
	}
	if h.logger != nil {
		h.logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logutil.LogAttr(c.Request.Context(),
			slog.String("tool", "query_customer"),
			slog.Int("count", len(customers)),
			slog.Int64("duration_ms", logutil.TimeNowMS()-start),
		)...)
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":      "query_customer",
		"count":     len(customers),
		"customers": customers,
	})
}

// ReadFile 读取 base 目录下的文件。
// 安全限制：拒绝路径穿越与越界访问，仅允许访问 base 内部文件。
// 错误场景：非法 JSON → 400；缺 path → 400；路径穿越/越界 → 403；文件不存在 → 404；读取失败 → 500。
func (h *DemoHandlers) ReadFile(c *gin.Context) {
	start := logutil.TimeNowMS()
	var payload struct {
		Path string `json:"path"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			if h.metrics != nil {
				h.metrics.RecordError("read_file_invalid_json")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file invalid json", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.Path == "" {
		if h.metrics != nil {
			h.metrics.RecordError("read_file_missing_path")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file missing path")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	baseAbs, err := filepath.Abs(h.base)
	if err != nil {
		if h.metrics != nil {
			h.metrics.RecordError("read_file_base_dir_unavailable")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "read_file base dir unavailable", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "base dir unavailable"})
		return
	}

	// 显式拒绝路径穿越意图。
	if strings.Contains(payload.Path, "..") {
		if h.metrics != nil {
			h.metrics.RecordError("read_file_path_traversal")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file path traversal blocked", logutil.LogAttr(c.Request.Context(), slog.String("path", payload.Path))...)
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "path traversal blocked"})
		return
	}

	target := filepath.Join(baseAbs, filepath.Clean(payload.Path))

	// 双保险：确保最终路径仍在 base 目录内。
	rel, err := filepath.Rel(baseAbs, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if h.metrics != nil {
			h.metrics.RecordError("read_file_path_outside_base")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file path outside base dir", logutil.LogAttr(c.Request.Context(), slog.String("path", payload.Path))...)
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "path outside base dir"})
		return
	}

	data, err := os.ReadFile(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if h.metrics != nil {
				h.metrics.RecordError("read_file_not_found")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "read_file not found", logutil.LogAttr(c.Request.Context(), slog.String("path", payload.Path))...)
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		if h.metrics != nil {
			h.metrics.RecordError("read_file_failed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "read_file failed", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read failed"})
		return
	}

	if h.metrics != nil {
		h.metrics.RecordTool("read_file")
	}
	if h.logger != nil {
		h.logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logutil.LogAttr(c.Request.Context(),
			slog.String("tool", "read_file"),
			slog.String("path", payload.Path),
			slog.Int64("duration_ms", logutil.TimeNowMS()-start),
		)...)
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":    "read_file",
		"path":    payload.Path,
		"content": string(data),
	})
}

// FetchURL 请求外部 URL，并返回经过脱敏的响应。
// 安全限制：域名白名单、SSRF（内网 IP）拦截、响应大小上限、敏感字段脱敏。
func (h *DemoHandlers) FetchURL(c *gin.Context) {
	start := logutil.TimeNowMS()
	var payload struct {
		URL string `json:"url"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			if h.metrics != nil {
				h.metrics.RecordError("fetch_url_invalid_json")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url invalid json", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.URL == "" {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_missing_url")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url missing url")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	u, err := url.Parse(payload.URL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_invalid_url")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url invalid url", logutil.LogAttr(c.Request.Context(), slog.String("url", payload.URL))...)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid url"})
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_scheme_not_allowed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url scheme not allowed", logutil.LogAttr(c.Request.Context(), slog.String("scheme", u.Scheme))...)
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "scheme not allowed"})
		return
	}

	host := u.Hostname()
	if !security.AllowedFetchHosts[host] {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_domain_not_allowed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url domain not allowed", logutil.LogAttr(c.Request.Context(), slog.String("host", host))...)
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "domain not allowed"})
		return
	}

	// SSRF 防护：解析域名并拒绝内网地址。
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_dns_failed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url dns failed", logutil.LogAttr(c.Request.Context(), slog.String("host", host), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "dns resolve failed"})
		return
	}
	for _, ip := range ips {
		if security.IsPrivateIP(ip) {
			if h.metrics != nil {
				h.metrics.RecordError("fetch_url_private_address")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url private address blocked", logutil.LogAttr(c.Request.Context(), slog.String("ip", ip.String()))...)
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "private address blocked"})
			return
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(payload.URL)
	if err != nil {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_request_failed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url request failed", logutil.LogAttr(c.Request.Context(), slog.String("url", payload.URL), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "request failed"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, model.MaxFetchBodyBytes+1))
	if err != nil {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_read_response_failed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "fetch_url read response failed", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "read response failed"})
		return
	}
	if int64(len(body)) > model.MaxFetchBodyBytes {
		if h.metrics != nil {
			h.metrics.RecordError("fetch_url_response_too_large")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "fetch_url response too large", logutil.LogAttr(c.Request.Context(), slog.String("url", payload.URL), slog.Int("size", len(body)))...)
		}
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "response too large"})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if h.metrics != nil {
		h.metrics.RecordTool("fetch_url")
	}
	if h.logger != nil {
		h.logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logutil.LogAttr(c.Request.Context(),
			slog.String("tool", "fetch_url"),
			slog.String("url", payload.URL),
			slog.Int("status", resp.StatusCode),
			slog.Int64("duration_ms", logutil.TimeNowMS()-start),
		)...)
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":         "fetch_url",
		"url":          payload.URL,
		"status":       resp.StatusCode,
		"content_type": contentType,
		"body":         security.RedactSensitive(body, contentType),
	})
}

// DeleteCustomer 删除指定客户（敏感工具，需 API Key）。
// 鉴权由 DemoAPIKeyAuth 中间件处理；未通过时不会进入本函数，下游 DELETE 不会执行。
// 错误场景：非法 JSON → 400；缺 id → 400；未配置数据库 → 503；超时 → 504；删除失败 → 500。
func (h *DemoHandlers) DeleteCustomer(c *gin.Context) {
	start := logutil.TimeNowMS()
	var payload struct {
		ID int64 `json:"id"`
	}
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			if h.metrics != nil {
				h.metrics.RecordError("delete_customer_invalid_json")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer invalid json", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
			return
		}
	}
	if payload.ID <= 0 {
		if h.metrics != nil {
			h.metrics.RecordError("delete_customer_missing_id")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer missing id")
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if h.db == nil {
		if h.metrics != nil {
			h.metrics.RecordError("delete_customer_db_unavailable")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelWarn, "delete_customer database unavailable")
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	tag, err := h.db.Exec(ctx, `DELETE FROM demo_customers WHERE id = $1`, payload.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			if h.metrics != nil {
				h.metrics.RecordError("delete_customer_timeout")
			}
			if h.logger != nil {
				h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "delete_customer timeout", logutil.LogAttr(c.Request.Context(), slog.Int64("id", payload.ID))...)
			}
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "operation timed out"})
			return
		}
		if h.metrics != nil {
			h.metrics.RecordError("delete_customer_failed")
		}
		if h.logger != nil {
			h.logger.LogAttrs(c.Request.Context(), slog.LevelError, "delete_customer failed", logutil.LogAttr(c.Request.Context(), slog.String("error", err.Error()))...)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}

	if h.metrics != nil {
		h.metrics.RecordTool("delete_customer")
	}
	if h.logger != nil {
		h.logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "tool called", logutil.LogAttr(c.Request.Context(),
			slog.String("tool", "delete_customer"),
			slog.Int64("id", payload.ID),
			slog.Bool("deleted", tag.RowsAffected() > 0),
			slog.Int64("duration_ms", logutil.TimeNowMS()-start),
		)...)
	}

	c.JSON(http.StatusOK, gin.H{
		"tool":    "delete_customer",
		"id":      payload.ID,
		"deleted": tag.RowsAffected() > 0,
	})
}
