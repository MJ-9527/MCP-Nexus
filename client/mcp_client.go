package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
<<<<<<< HEAD
	"io"
=======
	"fmt"
	"io"
	"net"
>>>>>>> origin/pull-request
	"net/http"
	"time"
)

<<<<<<< HEAD
var (
	ErrToolNotFound  = errors.New("tool not found")
	ErrToolOffline   = errors.New("tool is offline")
	ErrServerUnavailable = errors.New("server unavailable")
	ErrUpstreamTimeout   = errors.New("upstream timeout")
	ErrUpstreamError     = errors.New("upstream error")
)

// MCPClient talks to a downstream MCP server via HTTP.
type MCPClient struct {
	httpClient *http.Client
	timeout    time.Duration
}

func NewMCPClient(timeout time.Duration) *MCPClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &MCPClient{
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
	}
}

// Call forwards a tool invocation to the downstream server and returns the typed result.
// endpoint is the base URL of the MCP server (e.g. "http://localhost:9001").
// toolName is the tool identifier.
// args is the JSON-serialisable arguments map.
func (c *MCPClient) Call(ctx context.Context, endpoint, toolName string, args any) (any, error) {
	url := endpoint + "/tools/" + toolName + "/call"
	var bodyBuf bytes.Buffer
	if args != nil {
		if err := json.NewEncoder(&bodyBuf).Encode(args); err != nil {
			return nil, ErrUpstreamError
		}
	} else {
		bodyBuf.WriteString("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &bodyBuf)
	if err != nil {
		return nil, ErrUpstreamError
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, ErrUpstreamTimeout
		}
		return nil, ErrUpstreamError
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ErrUpstreamError
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ErrUpstreamError
	}
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, ErrUpstreamError
	}
	return result, nil
=======
// 传输层哨兵错误。service 层据此映射为业务错误码（TOOL_OFFLINE / UPSTREAM_TIMEOUT 等）。
var (
	ErrUpstreamTimeout   = errors.New("upstream timeout")
	ErrServerUnreachable = errors.New("upstream server unreachable")
	ErrBodyTooLarge      = errors.New("upstream response body too large")
	ErrUpstreamError     = errors.New("upstream error")
)

const (
	// DefaultConnectTimeout 连接建立超时（B13：三类超时之一）。
	// 短于请求超时，让连接故障快速失败，避免占用整体预算。
	DefaultConnectTimeout = 3 * time.Second
	// DefaultRequestTimeout 单次 HTTP 请求（含响应头）超时（B13：三类超时之一）。
	DefaultRequestTimeout = 5 * time.Second
	// DefaultTimeout 整体调用超时（含重试，B13：三类超时之一）。
	DefaultTimeout = 10 * time.Second
	// DefaultMaxBodyBytes 响应体大小上限，防止上游异常响应拖垮网关。
	DefaultMaxBodyBytes = 1 << 20 // 1MB
)

// MCPClient 下游 MCP Server 的 HTTP 客户端。
// 复用底层连接池；统一三类超时与响应体大小限制；透传网关 Header（request_id 等）；
// 集成按 endpoint 维度的熔断器与幂等请求重试（B13）。
type MCPClient struct {
	httpClient    *http.Client
	maxBodyBytes  int64
	defaultHeader map[string]string
	retry         RetryConfig
	breakers      *CircuitBreakerRegistry
}

func NewMCPClient(timeout time.Duration, maxBodyBytes int64) *MCPClient {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	// B13：明确区分连接超时与请求超时。
	// connectTimeout 取 min(timeout, DefaultConnectTimeout)，避免整体预算被连接故障吃光。
	connectTimeout := DefaultConnectTimeout
	if timeout < connectTimeout {
		connectTimeout = timeout
	}
	requestTimeout := timeout
	if requestTimeout > DefaultRequestTimeout {
		requestTimeout = DefaultRequestTimeout
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   connectTimeout, // 连接建立超时
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100, // 连接复用：最大空闲连接
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: requestTimeout, // 响应头超时
	}
	return &MCPClient{
		httpClient:   &http.Client{Transport: transport, Timeout: timeout},
		maxBodyBytes: maxBodyBytes,
		retry:        DefaultRetryConfig(),
		breakers:     NewCircuitBreakerRegistry(5, 30*time.Second),
	}
}

// SetRetry 注入重试配置（B13）。零值配置（MaxRetries=0）等价于关闭重试。
func (c *MCPClient) SetRetry(cfg RetryConfig) {
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 2 * time.Second
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 200 * time.Millisecond
	}
	c.retry = cfg
}

// SetCircuitBreakerRegistry 注入熔断器注册表（B13）。便于复用与可观测性查询。
func (c *MCPClient) SetCircuitBreakerRegistry(r *CircuitBreakerRegistry) {
	if r != nil {
		c.breakers = r
	}
}

// Breakers 暴露熔断器注册表，供可观测性接口查询（B14）。
func (c *MCPClient) Breakers() *CircuitBreakerRegistry {
	return c.breakers
}

// PostJSON 向下游发送 JSON 请求并返回原始响应体。
// 调用方负责将返回体反序列化并透传给上游请求方。
// POST 默认不重试（非幂等），但仍受熔断保护。
func (c *MCPClient) PostJSON(ctx context.Context, url string, payload any, headers map[string]string) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	respBody, status, err := c.doWithPolicy(ctx, http.MethodPost, url, body, headers)
	if err != nil {
		return respBody, err
	}
	if status < 200 || status > 299 {
		return respBody, fmt.Errorf("%w: http status %d", ErrUpstreamError, status)
	}
	return respBody, nil
}

// DoRequest 通用 HTTP 请求方法，供 OpenAPI 翻译器复用连接池与超时控制（B10）。
// 与 PostJSON 不同：支持任意 method；body 为已序列化的字节（可为 nil）；非 2xx
// 不视为传输错误——返回 body 与 status，由调用方按业务语义判定 IsError。
// 仅网络层故障（超时/不可达/响应体超限）返回 error，与 PostJSON 共用错误分类。
// B13：幂等方法（GET/HEAD/PUT/DELETE）受熔断保护且可重试。
func (c *MCPClient) DoRequest(ctx context.Context, method, url string, body []byte, headers map[string]string) ([]byte, int, error) {
	return c.doWithPolicy(ctx, method, url, body, headers)
}

// doWithPolicy 核心调用：熔断 + 重试 + 单次发送。
// 熔断检查放最前，避免已故障上游继续消耗请求预算；成功/失败均回灌熔断器。
// 重试仅对幂等方法 + 网络层错误触发，业务响应（含 5xx）不重试。
func (c *MCPClient) doWithPolicy(ctx context.Context, method, url string, body []byte, headers map[string]string) ([]byte, int, error) {
	cb := c.breakers.Get(url)
	if err := cb.Allow(); err != nil {
		return nil, 0, err
	}

	var lastBody []byte
	var lastStatus int
	var lastErr error

	for attempt := 0; ; attempt++ {
		// 上下文已取消（整体超时预算耗尽）：终止重试
		if ctx.Err() != nil {
			lastErr = fmt.Errorf("%w: %v", ErrUpstreamTimeout, ctx.Err())
			break
		}
		// 注意：respBody 与外层 body 命名隔离，避免下一轮重试传错请求体。
		respBody, status, err := c.sendOnce(ctx, method, url, body, headers)
		lastBody, lastStatus, lastErr = respBody, status, err

		if err == nil {
			// 成功（含 4xx/5xx 响应，已成功收到上游应答）
			// 仅 5xx 视为上游异常记熔断失败；4xx 是客户端错误，不影响上游健康
			if status >= 500 {
				cb.RecordFailure()
			} else {
				cb.RecordSuccess()
			}
			return respBody, status, nil
		}

		// 网络层错误：记熔断失败
		cb.RecordFailure()

		if !c.retry.ShouldRetry(attempt, method, err, ctx.Err() != nil) {
			return respBody, status, err
		}

		// 退避后重试。退避期间若上下文已到期则终止。
		delay := c.retry.BackoffDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			lastErr = fmt.Errorf("%w: %v", ErrUpstreamTimeout, ctx.Err())
			return lastBody, lastStatus, lastErr
		case <-timer.C:
		}
	}
	return lastBody, lastStatus, lastErr
}

// sendOnce 执行单次 HTTP 请求（无熔断无重试）。
func (c *MCPClient) sendOnce(ctx context.Context, method, url string, body []byte, headers map[string]string) ([]byte, int, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range c.defaultHeader {
		req.Header.Set(k, v)
	}
	for k, v := range headers { // 网关透传 Header（request_id 等）
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, classifyClientError(err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBodyBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: read body: %v", ErrUpstreamError, err)
	}
	if int64(len(respBody)) > c.maxBodyBytes {
		return nil, 0, fmt.Errorf("%w: limit %d bytes", ErrBodyTooLarge, c.maxBodyBytes)
	}
	return respBody, resp.StatusCode, nil
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
>>>>>>> origin/pull-request
}
