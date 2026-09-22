package service

import (
	"context"
	"errors"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

var (
	ErrGatewayInternal = errors.New("gateway internal error")
)

// GatewayService implements MCP tool discovery and invocation through the gateway.
type GatewayService struct {
	tools   repository.ToolRepository
	servers repository.ServerRepository
	mcp     *client.MCPClient
	now     func() time.Time
}

func NewGatewayService(tools repository.ToolRepository, servers repository.ServerRepository, mcp *client.MCPClient) *GatewayService {
	return &GatewayService{tools: tools, servers: servers, mcp: mcp, now: time.Now}
}

// ListTools returns only published tools whose owning server is active and online.
func (s *GatewayService) ListTools(ctx context.Context) ([]*model.MCPTool, error) {
	if s == nil || s.tools == nil || s.servers == nil {
		return nil, ErrGatewayInternal
	}
	all, err := s.tools.List(ctx, repository.ToolFilter{Published: boolPtr(true)})
	if err != nil {
		return nil, err
	}
	active := make([]*model.MCPTool, 0, len(all))
	for _, t := range all {
		server, err := s.servers.FindByID(ctx, t.ServerID)
		if err != nil {
			continue
		}
		if server.Status == "active" && server.HealthStatus == "online" {
			active = append(active, t)
		}
	}
	return active, nil
}

// CallTool resolves a tool by name, checks health, forwards the request, and returns the result.
func (s *GatewayService) CallTool(ctx context.Context, toolName string, args any) (map[string]any, error) {
	if s == nil || s.tools == nil || s.servers == nil || s.mcp == nil {
		return nil, ErrGatewayInternal
	}
	tools, err := s.tools.List(ctx, repository.ToolFilter{Name: toolName, Published: boolPtr(true)})
	if err != nil {
		return nil, client.ErrUpstreamError
	}
	if len(tools) == 0 {
		return nil, client.ErrToolNotFound
	}
	tool := tools[0]
	if !tool.Published {
		return nil, client.ErrToolOffline
	}
	server, err := s.servers.FindByID(ctx, tool.ServerID)
	if err != nil {
		return nil, client.ErrServerUnavailable
	}
	if server.Status != "active" || server.HealthStatus != "online" {
		return nil, client.ErrServerUnavailable
	}
	rawResult, err := s.mcp.Call(ctx, server.Endpoint, tool.Name, args)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"tool":  tool.Name,
		"result": rawResult,
	}
	if m, ok := rawResult.(map[string]any); ok {
		if data, hasData := m["data"]; hasData {
			out["data"] = data
		} else if res, hasRes := m["result"]; hasRes {
			out["data"] = res
		}
	}
	return out, nil
}

func boolPtr(b bool) *bool { return &b }
