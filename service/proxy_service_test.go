package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"MCP-Nexus/model"
	"MCP-Nexus/permission"
	"MCP-Nexus/repository"
)

func TestProxyService_ListAndCallTool(t *testing.T) {
	// 起一个假下游 MCP Server，模拟 /health 和 /tools/{name}/call
	// 响应体符合 MCP 协议：{"content":[...],"is_error":false}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
			return
		}
		if r.URL.Path == "/tools/get_weather/call" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"content":[{"type":"text","text":"sunny, 东莞"}],"is_error":false}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 1. 准备内存仓库
	serverRepo := repository.NewMemoryServerRepository()
	toolRepo := repository.NewMemoryToolRepository()

	testServer := &model.MCPServer{
		Name:         "weather-server",
		Endpoint:     ts.URL,
		Status:       "active",
		HealthStatus: "online",
	}
	if err := serverRepo.Create(context.Background(), testServer); err != nil {
		t.Fatalf("create server: %v", err)
	}

	toolWeather := &model.MCPTool{
		Name:         "get_weather",
		ServerID:     testServer.ID,
		Published:    true,
		HealthStatus: "online",
	}
	toolUser := &model.MCPTool{
		Name:         "get_user_info",
		ServerID:     testServer.ID,
		Published:    true,
		HealthStatus: "online",
	}
	toolRepo.TestPutTool(toolWeather)
	toolRepo.TestPutTool(toolUser)

	// 2. 使用 permission.MockClient
	permCli := permission.NewMockClient()
	// 设置角色权限：agent_role_01 只允许 get_weather
	permCli.MockData["agent_role_01"] = []string{"get_weather"}

	svc := NewProxyService(serverRepo, toolRepo, permCli, nil)

	// 3. ListTools 测试
	t.Run("ListTools_OnlyReturnPermittedTools", func(t *testing.T) {
		resp, err := svc.ListTools(context.Background(), "agent_role_01")
		if err != nil {
			t.Fatalf("ListTools error: %v", err)
		}
		if len(resp.Tools) != 1 || resp.Tools[0].Name != "get_weather" {
			t.Fatalf("expect [get_weather], got %+v", resp.Tools)
		}
		t.Log("✅ ListTools 权限过滤正常")
	})

	// 4. 允许调用工具
	t.Run("CallTool_Allowed", func(t *testing.T) {
		req := &model.McpToolCallRequest{
			Method:    "tools/call",
			ToolName:  "get_weather",
			Arguments: map[string]interface{}{"city": "东莞"},
		}
		res, err := svc.CallTool(context.Background(), "agent_role_01", 0, req)
		if err != nil {
			t.Fatalf("call get_weather failed: %v", err)
		}
		if len(res.Content) == 0 {
			t.Fatalf("expect non-empty content, got %+v", res)
		}
		t.Logf("✅ CallTool success: content=%v is_error=%v", res.Content, res.IsError)
	})

	// 5. 无权限拦截
	t.Run("CallTool_Denied", func(t *testing.T) {
		req := &model.McpToolCallRequest{
			Method:    "tools/call",
			ToolName:  "get_user_info",
			Arguments: map[string]interface{}{},
		}
		res, err := svc.CallTool(context.Background(), "agent_role_01", 0, req)
		if err == nil {
			t.Fatalf("expect permission denied error, res=%+v", res)
		}
		t.Logf("✅ 成功拦截无权限工具, err=%v", err)
	})
}
