package repository

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// chHTTP 封装 ClickHouse HTTP 接口的通用要素：地址归一化、库名、认证头与超时。
// 审计写入（ClickHouseAuditSink）与指标聚合（ClickHouseMetricsRepository）共用，
// 保证两处的连接参数、鉴权与错误处理保持一致。
type chHTTP struct {
	baseURL  string
	database string
	user     string
	password string
	client   *http.Client
}

// chIdentifier 校验库名/表名，避免配置写错导致 SQL 拼接出意外语句。
var chIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// newCHHTTP 构造共用客户端。addr 支持 "host:port" 或带 scheme；database 为空取默认值。
func newCHHTTP(addr, database, user, password string, timeout time.Duration) (*chHTTP, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("clickhouse addr is empty")
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	if database == "" {
		database = "mcp_analytics"
	}
	if !chIdentifier.MatchString(database) {
		return nil, fmt.Errorf("invalid clickhouse database identifier %q", database)
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &chHTTP{
		baseURL:  strings.TrimRight(addr, "/"),
		database: database,
		user:     user,
		password: password,
		client:   &http.Client{Timeout: timeout},
	}, nil
}

// qualified 返回 db.table 形式的限定名。
func (c *chHTTP) qualified(table string) string {
	return c.database + "." + table
}

// newRequest 构造请求：库名与 SQL 都放在 query 参数里（ClickHouse HTTP 约定）。
func (c *chHTTP) newRequest(ctx context.Context, sql string, body io.Reader) (*http.Request, error) {
	values := url.Values{}
	values.Set("database", c.database)
	values.Set("query", sql)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/?"+values.Encode(), body)
	if err != nil {
		return nil, fmt.Errorf("build clickhouse request: %w", err)
	}
	if c.user != "" {
		req.Header.Set("X-ClickHouse-User", c.user)
	}
	if c.password != "" {
		req.Header.Set("X-ClickHouse-Key", c.password)
	}
	return req, nil
}

// maxResponseBytes 读取响应体的上限，防止异常响应拖垮网关。
const maxResponseBytes = 4 << 20 // 4MB

func (c *chHTTP) do(req *http.Request) ([]byte, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clickhouse request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("clickhouse http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// execute 执行查询并返回响应文本（通常配合 FORMAT TabSeparated 使用）。
func (c *chHTTP) execute(ctx context.Context, sql string) ([]byte, error) {
	req, err := c.newRequest(ctx, sql, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// writeBody 以给定请求体执行语句（如 INSERT ... FORMAT JSONEachRow）。
func (c *chHTTP) writeBody(ctx context.Context, sql, contentType string, body []byte) error {
	req, err := c.newRequest(ctx, sql, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	_, err = c.do(req)
	return err
}
