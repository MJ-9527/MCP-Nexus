package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

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
}
