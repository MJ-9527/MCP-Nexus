package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// stubPermissionClient 固定角色/用户 → 工具授权映射，隔离权限存储。
type stubPermissionClient struct {
	roleAllowed map[string][]string
	userAllowed map[int64][]string
}

func (s *stubPermissionClient) GetAllowedToolNames(_ context.Context, role string, userID int64, action string) ([]string, error) {
	// 简化：call 与 view 共用同一授权集合；其余 action（publish/manage）不走工具级鉴权
	if action != PermissionActionCall && action != PermissionActionView {
		return nil, nil
	}
	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, name := range append(append([]string{}, s.roleAllowed[role]...), s.userAllowed[userID]...) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names, nil
}

func newFixture(t *testing.T) (*ProxyService, *repository.MemoryToolRepository, *repository.MemoryServerRepository) {
	t.Helper()
	serverRepo := repository.NewMemoryServerRepository()
	toolRepo := repository.NewMemoryToolRepository()
	permCli := &stubPermissionClient{roleAllowed: map[string][]string{
		"agent": {"query_sales", "ghost"},
	}, userAllowed: map[int64][]string{}}
	svc := &ProxyService{
		serverRepo:    serverRepo,
		toolRepo:      toolRepo,
		permissionCli: permCli,
		mcpClient:     client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes),
	}
	return svc, toolRepo, serverRepo
}

func seedOnline(t *testing.T, serverRepo *repository.MemoryServerRepository, toolRepo *repository.MemoryToolRepository, endpoint string) {
	t.Helper()
	srv := &model.MCPServer{ID: 1, Name: "demo", Endpoint: endpoint, Status: "active", HealthStatus: "online"}
	if err := serverRepo.Create(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
	tool := &model.MCPTool{ID: 1, ServerID: srv.ID, Name: "query_sales", Description: "查销售", Published: true, HealthStatus: "online"}
	if err := toolRepo.Create(context.Background(), tool); err != nil {
		t.Fatal(err)
	}
}

func TestListToolsFiltersOfflineServerAndUnpublishedTool(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")

	// 未发布工具不应出现在发现列表
	if err := toolRepo.Create(context.Background(), &model.MCPTool{ID: 2, ServerID: 1, Name: "secret_tool", Published: false, HealthStatus: "online"}); err != nil {
		t.Fatal(err)
	}
	// 无权限工具不应出现
	if err := toolRepo.Create(context.Background(), &model.MCPTool{ID: 3, ServerID: 1, Name: "query_inventory", Published: true, HealthStatus: "online"}); err != nil {
		t.Fatal(err)
	}
	// 离线 Server 上的工具不应出现
	srv2 := &model.MCPServer{ID: 2, Name: "down", Endpoint: "http://down", Status: "active", HealthStatus: "offline"}
	if err := serverRepo.Create(context.Background(), srv2); err != nil {
		t.Fatal(err)
	}
	if err := toolRepo.Create(context.Background(), &model.MCPTool{ID: 4, ServerID: 2, Name: "query_sales_v2", Published: true, HealthStatus: "online"}); err != nil {
		t.Fatal(err)
	}

	resp, err := svc.ListTools(context.Background(), "agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Tools) != 1 || resp.Tools[0].Name != "query_sales" {
		t.Fatalf("期望仅返回 query_sales，实际 %+v", resp.Tools)
	}
}

func TestCallToolSuccessPassthrough(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tools/query_sales/call" {
			t.Fatalf("调用路径错误: %s", r.URL.Path)
		}
		if r.Header.Get("request_id") == "" {
			t.Error("request_id 未透传到下游")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method: "tools/call", ToolName: "query_sales", Arguments: map[string]interface{}{"month": "2026-08"},
	}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError || len(resp.Content) != 1 || resp.Content[0]["text"] != "ok" {
		t.Fatalf("透传结果不符: %+v", resp)
	}
}

func TestCallToolErrors(t *testing.T) {
	cases := []struct {
		name    string
		seed    func(t *testing.T, toolRepo *repository.MemoryToolRepository, serverRepo *repository.MemoryServerRepository)
		role    string
		wantErr error
	}{
		{"工具不存在", func(t *testing.T, toolRepo *repository.MemoryToolRepository, serverRepo *repository.MemoryServerRepository) {
		}, "agent", ErrToolNotFound},
		{"工具未发布视为不存在", func(t *testing.T, toolRepo *repository.MemoryToolRepository, serverRepo *repository.MemoryServerRepository) {
			_ = toolRepo.Create(context.Background(), &model.MCPTool{ID: 9, ServerID: 1, Name: "ghost", Published: false, HealthStatus: "online"})
		}, "agent", ErrToolNotFound},
		{"工具离线", func(t *testing.T, toolRepo *repository.MemoryToolRepository, serverRepo *repository.MemoryServerRepository) {
			_ = toolRepo.Create(context.Background(), &model.MCPTool{ID: 9, ServerID: 1, Name: "ghost", Published: true, HealthStatus: "offline"})
		}, "agent", ErrToolOffline},
		{"无权限", func(t *testing.T, toolRepo *repository.MemoryToolRepository, serverRepo *repository.MemoryServerRepository) {
		}, "anonymous", ErrPermissionDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, toolRepo, serverRepo := newFixture(t)
			seedOnline(t, serverRepo, toolRepo, "http://unused")
			tc.seed(t, toolRepo, serverRepo)
			_, err := svc.CallTool(context.Background(), tc.role, 0, &model.McpToolCallRequest{ToolName: "ghost"}, "req-1")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("期望 %v，实际 %v", tc.wantErr, err)
			}
		})
	}
}

func TestCallToolUnknownHealthPasses(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)
	// 注册默认 unknown：未检测不应拦截调用（ghost 已在授权名单中）
	_ = toolRepo.Create(context.Background(), &model.MCPTool{ID: 2, ServerID: 1, Name: "ghost", Published: true, HealthStatus: "unknown"})

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{ToolName: "ghost"}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError || len(resp.Content) != 1 {
		t.Fatalf("unknown 健康状态应放行: %+v", resp)
	}
}

func TestCallToolServerOffline(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")
	_ = serverRepo.UpdateHealth(context.Background(), 1, "offline", time.Now())

	_, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{ToolName: "query_sales"}, "req-1")
	if !errors.Is(err, ErrServerUnavailable) {
		t.Fatalf("期望 ErrServerUnavailable，实际 %v", err)
	}
}

func TestCallToolUpstreamTimeout(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	// 下游 200ms 才响应，客户端超时 50ms → UPSTREAM_TIMEOUT
	svc.mcpClient = client.NewMCPClient(50*time.Millisecond, client.DefaultMaxBodyBytes)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)

	_, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{ToolName: "query_sales"}, "req-1")
	if !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("期望 ErrUpstreamTimeout，实际 %v", err)
	}
}

func TestCallToolUpstreamBadStatus(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)

	_, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{ToolName: "query_sales"}, "req-1")
	if !errors.Is(err, ErrUpstreamError) {
		t.Fatalf("期望 ErrUpstreamError，实际 %v", err)
	}
}

// B6：用户直授（user_id 维度）即使无角色授权也应生效
func TestCallToolUserDirectGrant(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)
	// user 42 直接授权 query_inventory（不在任何角色名单中）
	svc.permissionCli.(*stubPermissionClient).userAllowed[42] = []string{"query_inventory"}
	_ = toolRepo.Create(context.Background(), &model.MCPTool{ID: 3, ServerID: 1, Name: "query_inventory", Published: true, HealthStatus: "online"})

	if _, err := svc.CallTool(context.Background(), "anonymous", 42, &model.McpToolCallRequest{ToolName: "query_inventory"}, "req-1"); err != nil {
		t.Fatalf("用户直授应放行: %v", err)
	}
	// 其他用户无直授仍被拒
	_, err := svc.CallTool(context.Background(), "anonymous", 43, &model.McpToolCallRequest{ToolName: "query_inventory"}, "req-1")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("无直授用户期望 ErrPermissionDenied，实际 %v", err)
	}
}

// B6：拒绝原因应包含用户、角色与工具信息
func TestCallToolDeniedReason(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")
	_ = toolRepo.Create(context.Background(), &model.MCPTool{ID: 3, ServerID: 1, Name: "query_inventory", Published: true, HealthStatus: "online"})

	_, err := svc.CallTool(context.Background(), "agent", 7, &model.McpToolCallRequest{ToolName: "query_inventory"}, "req-1")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("期望 ErrPermissionDenied，实际 %v", err)
	}
	for _, want := range []string{"user 7", "agent", "query_inventory"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("拒绝原因缺少 %q: %v", want, err)
		}
	}
}

// B6：工具发现取 view/call 并集，仅授 view 的工具也应可见
func TestListToolsViewUnionCall(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")
	_ = toolRepo.Create(context.Background(), &model.MCPTool{ID: 3, ServerID: 1, Name: "readonly_report", Published: true, HealthStatus: "online"})
	// 简化 stub 中 view 与 call 共用授权集合，这里直接把只读工具加入 agent 名单验证可见性
	svc.permissionCli.(*stubPermissionClient).roleAllowed["agent"] = []string{"query_sales", "ghost", "readonly_report"}

	resp, err := svc.ListTools(context.Background(), "agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(resp.Tools))
	for _, tool := range resp.Tools {
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "readonly_report") {
		t.Fatalf("view/call 并集应包含 readonly_report: %v", names)
	}
}
