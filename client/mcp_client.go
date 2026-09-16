package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// 传输层哨兵错误。service 层据此映射为业务错误码（TOOL_OFFLINE / UPSTREAM_TIMEOUT 等）。
var (
	ErrUpstreamTimeout   = errors.New("upstream timeout")
	ErrServerUnreachable = errors.New("upstream server unreachable")
	ErrBodyTooLarge      = errors.New("upstream response body too large")
	ErrUpstreamError     = errors.New("upstream error")
)

const (
	// DefaultTimeout 单次下游调用的整体超时（连接 + 请求 + 响应）。
	DefaultTimeout = 10 * time.Second
	// DefaultMaxBodyBytes 响应体大小上限，防止上游异常响应拖垮网关。
	DefaultMaxBodyBytes = 1 << 20 // 1MB
)

// MCPClient 下游 MCP Server 的 HTTP 客户端。
// 复用底层连接池；统一超时与响应体大小限制；透传网关 Header（request_id 等）。
type MCPClient struct {
	httpClient    *http.Client
	maxBodyBytes  int64
	defaultHeader map[string]string
}

func NewMCPClient(timeout time.Duration, maxBodyBytes int64) *MCPClient {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second, // 连接建立超时
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100, // 连接复用：最大空闲连接
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout, // 响应头超时
	}
	return &MCPClient{
		httpClient:   &http.Client{Transport: transport, Timeout: timeout},
		maxBodyBytes: maxBodyBytes,
	}
}

// PostJSON 向下游发送 JSON 请求并返回原始响应体。
// 调用方负责将返回体反序列化并透传给上游请求方。
func (c *MCPClient) PostJSON(ctx context.Context, url string, payload any, headers map[string]string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range c.defaultHeader {
		req.Header.Set(k, v)
	}
	for k, v := range headers { // 网关透传 Header（request_id 等）
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyClientError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrUpstreamError, err)
	}
	if int64(len(respBody)) > c.maxBodyBytes {
		return nil, fmt.Errorf("%w: limit %d bytes", ErrBodyTooLarge, c.maxBodyBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return respBody, fmt.Errorf("%w: http status %d", ErrUpstreamError, resp.StatusCode)
	}
	return respBody, nil
}

// classifyClientError 将网络层错误归类为超时 / 不可达两类。
func classifyClientError(err error) error {
	// 上下文超时（整体调用超时）或网络超时
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, err)
	}
	// 连接被拒 / 主机不可达 / DNS 失败
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return fmt.Errorf("%w: %v", ErrServerUnreachable, err)
	}
	return fmt.Errorf("%w: %v", ErrServerUnreachable, err)
}
