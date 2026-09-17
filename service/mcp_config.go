package service

import (
	"context"
	"fmt"

	"MCP-Nexus/model"
)

// McpConfigService 生成外部 Agent 接入配置（B9）。
// 配置与网关真实路由一一对应，工具清单复用 ListTools 的可见性语义（权限 ∩ 发布 ∩ 在线）。
type McpConfigService struct {
	proxy    *ProxyService
	tokenTTL string
}

func NewMcpConfigService(proxy *ProxyService, tokenTTL string) *McpConfigService {
	return &McpConfigService{proxy: proxy, tokenTTL: tokenTTL}
}

// Generate 生成当前账号的接入配置。baseURL 为网关对外可达地址（如 http://192.168.1.5:8080），
// 不含尾斜杠。
func (s *McpConfigService) Generate(ctx context.Context, role string, userID int64, baseURL string) (*model.McpConfigResponse, error) {
	list, err := s.proxy.ListTools(ctx, role, userID)
	if err != nil {
		return nil, fmt.Errorf("list tools for config: %w", err)
	}
	tools := list.Tools
	if tools == nil {
		tools = []model.McpToolView{}
	}
	endpoint := baseURL + "/mcp"
	return &model.McpConfigResponse{
		Protocol: "custom-rest",
		Endpoint: endpoint,
		Auth: model.McpAuthGuide{
			Type:       "bearer",
			TokenTTL:   s.tokenTTL,
			LoginPath:  "POST /api/auth/login  body: {\"username\":\"...\",\"password\":\"...\"}",
			HeaderName: "Authorization",
			Example:    fmt.Sprintf(`curl -X POST %s/api/auth/login -H "Content-Type: application/json" -d '{"username":"agent","password":"***"}'`, baseURL),
		},
		Operations: s.operations(endpoint),
		Tools:      tools,
	}, nil
}

// operations 返回发现/调用两个网关操作的调用说明。
func (s *McpConfigService) operations(endpoint string) []model.McpOperationDoc {
	return []model.McpOperationDoc{
		{
			Name:            "list_tools",
			Method:          "GET",
			Path:            "/mcp/tools",
			Description:     "发现当前账号可调用的工具（含 JSON Schema 入参说明）",
			RequestExample:  fmt.Sprintf(`curl %s/tools -H "Authorization: Bearer <token>"`, endpoint),
			ResponseExample: `{"code":"OK","data":{"tools":[{"name":"query_sales","description":"...","input_schema":{...}}]}}`,
		},
		{
			Name:            "call_tool",
			Method:          "POST",
			Path:            "/mcp/tools/{toolName}/call",
			Description:     "调用指定工具，arguments 按 input_schema 传参；超时 10s，限流按角色配额",
			RequestExample:  fmt.Sprintf(`curl -X POST %s/tools/query_sales/call -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '{"arguments":{"month":"2026-08","region":"华东"}}'`, endpoint),
			ResponseExample: `{"code":"OK","data":{"content":[{"type":"text","text":"..."}],"is_error":false}}`,
		},
	}
}
