package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrUnhealthy 表示下游服务返回了非 2xx 状态码（服务可达但异常）。
var ErrUnhealthy = errors.New("downstream unhealthy")

// HealthClient 负责探测下游 MCP Server 的 /health 接口。
type HealthClient struct {
	client *http.Client
}

// NewHealthClient 创建一个带超时的健康检查客户端，默认 5 秒。
func NewHealthClient(timeout time.Duration) *HealthClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &HealthClient{client: &http.Client{Timeout: timeout}}
}

// HealthResult 是一次健康探测的结果。
type HealthResult struct {
	Latency time.Duration
	Err     error
}

// Check 请求 {endpoint}/health，返回探测结果（Err 为 nil 表示在线）。
func (c *HealthClient) Check(ctx context.Context, endpoint string) HealthResult {
	start := time.Now()
	url := strings.TrimRight(endpoint, "/") + "/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return HealthResult{Latency: time.Since(start), Err: err}
	}

	resp, err := c.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return HealthResult{Latency: latency, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HealthResult{Latency: latency, Err: fmt.Errorf("%w: status %d", ErrUnhealthy, resp.StatusCode)}
	}

	return HealthResult{Latency: latency}
}
