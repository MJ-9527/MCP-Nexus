package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// CallTimeout 单次工具调用的整体超时（网关侧保护，防止上游故障长期占用资源）。
const CallTimeout = 10 * time.Second

// PermissionClient 对接权限服务，用于查询角色允许调用的工具列表。
type PermissionClient interface {
	GetAllowedToolNamesByRole(ctx context.Context, role string) ([]string, error)
}

type ProxyService struct {
	serverRepo    repository.ServerRepository
	toolRepo      repository.ToolRepository
	permissionCli PermissionClient
	mcpClient     *client.MCPClient
}

func NewProxyService(serverRepo repository.ServerRepository,
	toolRepo repository.ToolRepository,
	permCli PermissionClient) *ProxyService {
	return &ProxyService{
		serverRepo:    serverRepo,
		toolRepo:      toolRepo,
		permissionCli: permCli,
		mcpClient:     client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes),
	}
}

// ListTools 工具发现：仅返回【Server 在线 + 工具已发布 + 角色有权限】的工具（B1/B3）。
func (s *ProxyService) ListTools(ctx context.Context, role string) (*model.McpListToolsResponse, error) {
	allowedToolNames, err := s.permissionCli.GetAllowedToolNamesByRole(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("query permission error: %w", err)
	}
	allowed := make(map[string]bool, len(allowedToolNames))
	for _, name := range allowedToolNames {
		allowed[name] = true
	}

	// 仅已发布工具（下线工具不出现在发现列表）
	published := true
	tools, err := s.toolRepo.List(ctx, repository.ToolFilter{Published: &published})
	if err != nil {
		return nil, fmt.Errorf("list tools error: %w", err)
	}

	servers, err := s.serverRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list servers error: %w", err)
	}
	onlineServerIDs := make(map[int64]bool, len(servers))
	for _, srv := range servers {
		if srv.HealthStatus == "online" {
			onlineServerIDs[srv.ID] = true
		}
	}

	viewList := make([]model.McpToolView, 0)
	for _, t := range tools {
		if onlineServerIDs[t.ServerID] && allowed[t.Name] {
			viewList = append(viewList, model.McpToolView{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}
	return &model.McpListToolsResponse{Tools: viewList}, nil
}

// CallTool 调用前检查（B3）→ 转发下游（B2）→ 统一业务错误（B4）。
func (s *ProxyService) CallTool(ctx context.Context, role string, req *model.McpToolCallRequest, requestID string) (*model.McpToolCallResponse, error) {
	// 1. RBAC：角色是否有权调用该工具
	allowedToolNames, err := s.permissionCli.GetAllowedToolNamesByRole(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("query permission error: %w", err)
	}
	hasPermission := false
	for _, name := range allowedToolNames {
		if name == req.ToolName {
			hasPermission = true
			break
		}
	}
	if !hasPermission {
		return nil, ErrPermissionDenied
	}

	// 2. 工具存在且已发布
	tool, err := s.toolRepo.FindPublishedByName(ctx, req.ToolName)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrToolNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find tool error: %w", err)
	}

	// 3. 工具未被明确下线（unknown=未检测可放行；只有明确 offline 才拒绝）
	if tool.HealthStatus == "offline" {
		return nil, fmt.Errorf("%w: tool %s health is %s", ErrToolOffline, tool.Name, tool.HealthStatus)
	}

	// 4. 所属 Server 存在、已激活且健康在线
	server, err := s.serverRepo.FindByID(ctx, tool.ServerID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrServerNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find server error: %w", err)
	}
	if server.Status != "active" {
		return nil, fmt.Errorf("%w: server %s status is %s", ErrServerUnavailable, server.Name, server.Status)
	}
	if server.HealthStatus != "online" {
		return nil, fmt.Errorf("%w: server %s health is %s", ErrServerUnavailable, server.Name, server.HealthStatus)
	}

	// 5. 整体超时保护 + 转发（响应透传）
	callCtx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()

	respBody, err := s.mcpClient.PostJSON(callCtx, server.Endpoint, req, map[string]string{
		"request_id": requestID,
	})
	if err != nil {
		return nil, mapUpstreamError(err)
	}

	var callResp model.McpToolCallResponse
	if err := json.Unmarshal(respBody, &callResp); err != nil {
		return nil, fmt.Errorf("%w: parse upstream response: %v", ErrUpstreamError, err)
	}
	return &callResp, nil
}

// mapUpstreamError 将传输层错误映射为网关业务错误（B4）。
func mapUpstreamError(err error) error {
	switch {
	case errors.Is(err, client.ErrUpstreamTimeout):
		return fmt.Errorf("%w: %v", ErrUpstreamTimeout, errors.Unwrap(err))
	case errors.Is(err, client.ErrServerUnreachable):
		return fmt.Errorf("%w: %v", ErrServerUnavailable, errors.Unwrap(err))
	case errors.Is(err, client.ErrBodyTooLarge):
		return fmt.Errorf("%w: %v", ErrUpstreamError, errors.Unwrap(err))
	default:
		return fmt.Errorf("%w: %v", ErrUpstreamError, err)
	}
}
