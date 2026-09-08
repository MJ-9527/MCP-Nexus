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

type ProxyService struct {
	serverRepo repository.ServerRepository
	toolRepo   repository.ToolRepository
	httpClient *http.Client
}

func NewProxyService(serverRepo repository.ServerRepository, toolRepo repository.ToolRepository) *ProxyService {
	return &ProxyService{
		serverRepo: serverRepo,
		toolRepo:   toolRepo,
		httpClient: &http.Client{},
	}
}

// ListTools 获取所有健康在线的工具列表
func (s *ProxyService) ListTools(ctx context.Context) (*model.McpListToolsResponse, error) {
	tools, err := s.toolRepo.ListAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all tools error: %w", err)
	}

	servers, err := s.serverRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list server error: %w", err)
	}

	// 构建在线服务ID集合，业务过滤逻辑放在service
	onlineServerIDs := make(map[int64]bool)
	for _, srv := range servers {
		if srv.HealthStatus == "online" {
			onlineServerIDs[srv.ID] = true
		}
	}

	var viewList []model.McpToolView
	for _, t := range tools {
		if onlineServerIDs[t.ServerID] {
			viewList = append(viewList, model.McpToolView{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			})
		}
	}

	return &model.McpListToolsResponse{Tools: viewList}, nil
}

// CallTool 代理转发工具调用
func (s *ProxyService) CallTool(ctx context.Context, req *model.McpToolCallRequest) (*model.McpToolCallResponse, error) {
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
