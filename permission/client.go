package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// PermissionClient 对接队友的权限服务，用于查询角色允许的工具列表
type PermissionClient interface {
	GetAllowedToolNamesByRole(ctx context.Context, role string) ([]string, error)
}

// Client 真实http客户端，用来请求权限服务
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// GetAllowedToolNamesByRole 查询角色可用工具列表
func (c *Client) GetAllowedToolNamesByRole(ctx context.Context, role string) ([]string, error) {
	url := fmt.Sprintf("%s/api/permission/tools?role=%s", c.baseURL, role)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build req err: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errors.New("permission service timeout")
		}
		return nil, fmt.Errorf("call perm service err: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("perm service status %d", resp.StatusCode)
	}

	var res struct {
		Tools []string `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("parse perm resp err: %w", err)
	}

	return res.Tools, nil
}

// 编译校验，确认实现接口
var _ PermissionClient = (*Client)(nil)
