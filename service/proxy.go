package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// PermissionClient 对接队友的权限服务，用于查询角色允许的工具列表
type PermissionClient interface {
	GetAllowedToolNamesByRole(ctx context.Context, role string) ([]string, error)
}

type ProxyService struct {
	serverRepo    repository.ServerRepository
	toolRepo      repository.ToolRepository
	permissionCli PermissionClient
	httpClient    *http.Client
}

func NewProxyService(serverRepo repository.ServerRepository,
	toolRepo repository.ToolRepository,
	permCli PermissionClient) *ProxyService {
	return &ProxyService{
		serverRepo:    serverRepo,
		toolRepo:      toolRepo,
		permissionCli: permCli,
		httpClient:    &http.Client{},
	}
}

// ListTools 获取【在线服务 + 当前角色有权限】的工具列表
func (s *ProxyService) ListTools(ctx context.Context, role string) (*model.McpListToolsResponse, error) {
	// 查询当前角色允许访问的工具名集合
	allowedToolNames, err := s.permissionCli.GetAllowedToolNamesByRole(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("query permission error: %w", err)
	}
	allowedToolMap := make(map[string]bool)
	for _, name := range allowedToolNames {
		allowedToolMap[name] = true
	}

	tools, err := s.toolRepo.ListAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all tools error: %w", err)
	}

	servers, err := s.serverRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list server error: %w", err)
	}
	onlineServerIDs := make(map[int64]bool)
	for _, srv := range servers {
		if srv.HealthStatus == "online" {
			onlineServerIDs[srv.ID] = true
		}
	}

	//双重过滤：服务在线 && 角色拥有权限
	var viewList []model.McpToolView
	for _, t := range tools {
		if onlineServerIDs[t.ServerID] && allowedToolMap[t.Name] {
			viewList = append(viewList, model.McpToolView{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	return &model.McpListToolsResponse{Tools: viewList}, nil
}

// CallTool 代理转发工具调用，新增RBAC权限前置校验
func (s *ProxyService) CallTool(ctx context.Context, role string, req *model.McpToolCallRequest) (*model.McpToolCallResponse, error) {
	// RBAC权限校验
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
		return nil, errors.New("permission denied: agent cannot call this tool")
	}

	tool, err := s.toolRepo.FindToolByName(ctx, req.ToolName)
	if err != nil {
		return nil, fmt.Errorf("find tool error: %w", err)
	}

	server, err := s.serverRepo.FindByID(ctx, tool.ServerID)
	if err != nil {
		return nil, fmt.Errorf("find mcp server error: %w", err)
	}

	if server.HealthStatus != "online" {
		return nil, errors.New("target mcp server is offline")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request error: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, server.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build http request error: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("forward request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upstream body error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream http status %d, body:%s", resp.StatusCode, string(bodyBytes))
	}

	var callResp model.McpToolCallResponse
	if err := json.Unmarshal(bodyBytes, &callResp); err != nil {
		return nil, fmt.Errorf("parse upstream response error: %w", err)
	}

	return &callResp, nil
}
