package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// B12 端到端校验：在 ProxyService.CallTool 链路中校验失败应可预测地返回 400 类错误，
// 且不触发任何下游调用（atomic 计数验证 upstream hit 为 0）。

// newB12Fixture 构造一个带计数器的下游 stub，便于断言「未调用上游」。
func newB12Fixture(t *testing.T) (*ProxyService, *repository.MemoryToolRepository, *repository.MemoryServerRepository, *int64) {
	t.Helper()
	var upstreamHits int64
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&upstreamHits, 1)
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	t.Cleanup(downstream.Close)

	serverRepo := repository.NewMemoryServerRepository()
	toolRepo := repository.NewMemoryToolRepository()
	permCli := &stubPermissionClient{roleAllowed: map[string][]string{
		"agent": {"query_sales"},
	}, userAllowed: map[int64][]string{}}
	svc := &ProxyService{
		serverRepo:    serverRepo,
		toolRepo:      toolRepo,
		permissionCli: permCli,
		mcpClient:     client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes),
	}
	// 在线 server + 一个带 schema 的工具
	srv := &model.MCPServer{ID: 1, Name: "demo", Endpoint: downstream.URL, Status: "active", HealthStatus: "online"}
	if err := serverRepo.Create(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
	tool := &model.MCPTool{
		ID: 1, ServerID: srv.ID, Name: "query_sales", Published: true, HealthStatus: "online",
		Version: "1.0.0",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{"month":{"type":"string"},"limit":{"type":"integer"}},
			"required":["month"]
		}`),
	}
	if err := toolRepo.Create(context.Background(), tool); err != nil {
		t.Fatal(err)
	}
	return svc, toolRepo, serverRepo, &upstreamHits
}

func TestCallTool_MissingRequiredReturns400AndSkipsUpstream(t *testing.T) {
	svc, _, _, upstreamHits := newB12Fixture(t)

	_, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"limit": float64(10), // 缺 month
		},
	}, "req-b12-1")

	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("期望 ErrInvalidArguments，实际 %v", err)
	}
	if atomic.LoadInt64(upstreamHits) != 0 {
		t.Fatalf("校验失败不应调用下游，实际命中 %d 次", atomic.LoadInt64(upstreamHits))
	}
}

func TestCallTool_TypeMismatchReturns400AndSkipsUpstream(t *testing.T) {
	svc, _, _, upstreamHits := newB12Fixture(t)

	_, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"month": 202608, // 非字符串
			"limit": float64(10),
		},
	}, "req-b12-2")

	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("期望 ErrInvalidArguments，实际 %v", err)
	}
	if atomic.LoadInt64(upstreamHits) != 0 {
		t.Fatalf("校验失败不应调用下游，实际命中 %d 次", atomic.LoadInt64(upstreamHits))
	}
}

func TestCallTool_VersionMismatchReturns400AndSkipsUpstream(t *testing.T) {
	svc, _, _, upstreamHits := newB12Fixture(t)

	_, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"month":   "2026-08",
			"version": "2.0.0", // 与 tool.Version=1.0.0 不匹配
		},
	}, "req-b12-3")

	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("期望 ErrVersionMismatch，实际 %v", err)
	}
	if atomic.LoadInt64(upstreamHits) != 0 {
		t.Fatalf("版本校验失败不应调用下游，实际命中 %d 次", atomic.LoadInt64(upstreamHits))
	}
}

func TestCallTool_ValidArgsCallUpstream(t *testing.T) {
	// 正向回归：合法参数 + 一致版本应正常调用上游
	svc, _, _, upstreamHits := newB12Fixture(t)

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"month":   "2026-08",
			"limit":   float64(10),
			"version": "1.0.0",
		},
	}, "req-b12-4")

	if err != nil {
		t.Fatalf("合法参数应调用成功，实际 %v", err)
	}
	if resp == nil || len(resp.Content) != 1 || resp.Content[0]["text"] != "ok" {
		t.Fatalf("上游响应应透传，实际 %+v", resp)
	}
	if atomic.LoadInt64(upstreamHits) != 1 {
		t.Fatalf("合法调用应命中下游 1 次，实际 %d 次", atomic.LoadInt64(upstreamHits))
	}
}

func TestCallTool_NoClientVersionStillPasses(t *testing.T) {
	// 客户端不传 version：版本校验跳过，参数校验通过 → 调用上游
	svc, _, _, upstreamHits := newB12Fixture(t)

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"month": "2026-08",
		},
	}, "req-b12-5")

	if err != nil {
		t.Fatalf("未传 version 应放行，实际 %v", err)
	}
	if resp == nil || len(resp.Content) != 1 {
		t.Fatalf("上游响应应透传，实际 %+v", resp)
	}
	if atomic.LoadInt64(upstreamHits) != 1 {
		t.Fatalf("应命中下游 1 次，实际 %d 次", atomic.LoadInt64(upstreamHits))
	}
}

// 无 schema 的工具应完全跳过参数校验
func TestCallTool_NoSchemaSkipsValidation(t *testing.T) {
	svc, toolRepo, _, upstreamHits := newB12Fixture(t)

	// 新增一个无 schema 工具
	if err := toolRepo.Create(context.Background(), &model.MCPTool{
		ID: 2, ServerID: 1, Name: "noop", Published: true, HealthStatus: "online",
		InputSchema: nil, // 无 schema
	}); err != nil {
		t.Fatal(err)
	}
	svc.permissionCli.(*stubPermissionClient).roleAllowed["agent"] = append(
		svc.permissionCli.(*stubPermissionClient).roleAllowed["agent"], "noop")

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:    "tools/call",
		ToolName:  "noop",
		Arguments: map[string]interface{}{"any": "thing"},
	}, "req-b12-6")

	if err != nil {
		t.Fatalf("无 schema 应跳过校验并放行，实际 %v", err)
	}
	if resp == nil || len(resp.Content) != 1 {
		t.Fatalf("上游响应应透传，实际 %+v", resp)
	}
	if atomic.LoadInt64(upstreamHits) != 1 {
		t.Fatalf("应命中下游 1 次，实际 %d 次", atomic.LoadInt64(upstreamHits))
	}
}
