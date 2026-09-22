package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

func newImporter() (*OpenAPIImporter, *repository.MemoryServerRepository, *repository.MemoryToolRepository, *repository.MemoryOpenAPISpecRepository) {
	srvRepo := repository.NewMemoryServerRepository()
	toolRepo := repository.NewMemoryToolRepository()
	specRepo := repository.NewMemoryOpenAPISpecRepository()
	return NewOpenAPIImporter(srvRepo, toolRepo, specRepo), srvRepo, toolRepo, specRepo
}

func seedOpenAPIServer(t *testing.T, srvRepo *repository.MemoryServerRepository, endpoint string) int64 {
	t.Helper()
	srv := &model.MCPServer{ID: 1, Name: "petstore", Endpoint: endpoint, Version: "1.0.0", Status: "active", HealthStatus: "online"}
	if err := srvRepo.Create(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
	return srv.ID
}

func TestOpenAPIImporter_CreateTools(t *testing.T) {
	imp, srvRepo, toolRepo, specRepo := newImporter()
	srvID := seedOpenAPIServer(t, srvRepo, "http://petstore.example.com")

	result, err := imp.Import(context.Background(), srvID, model.ImportOpenAPIRequest{
		Spec:      json.RawMessage(petStoreSpec),
		Published: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 {
		t.Fatalf("期望导入 4 个，实际 %d", result.Total)
	}
	// 工具应入库且 category=openapi
	tools, _, _ := toolRepo.List(context.Background(), repository.ToolFilter{ServerID: &srvID})
	if len(tools) != 4 {
		t.Fatalf("工具入库数量 %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Category != "openapi" {
			t.Errorf("工具 %s category 应为 openapi", tool.Name)
		}
		if !tool.Published {
			t.Errorf("工具 %s 应已发布", tool.Name)
		}
	}
	// spec 应入库
	specs, _ := specRepo.ListByServerID(context.Background(), srvID)
	if len(specs) != 4 {
		t.Fatalf("spec 入库数量 %d", len(specs))
	}
}

func TestOpenAPIImporter_Idempotent(t *testing.T) {
	imp, srvRepo, toolRepo, specRepo := newImporter()
	srvID := seedOpenAPIServer(t, srvRepo, "http://petstore.example.com")

	req := model.ImportOpenAPIRequest{Spec: json.RawMessage(petStoreSpec)}
	// 第一次导入
	r1, err := imp.Import(context.Background(), srvID, req)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Total != 4 {
		t.Fatalf("首次导入应 4 个，实际 %d", r1.Total)
	}
	// 第二次导入：应复用工具（action=updated），不重复创建
	r2, err := imp.Import(context.Background(), srvID, req)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Total != 4 {
		t.Fatalf("二次导入应 4 个，实际 %d", r2.Total)
	}
	for _, e := range r2.Imported {
		if e.Action != "updated" {
			t.Errorf("二次导入工具 %s 应为 updated，实际 %s", e.Name, e.Action)
		}
	}
	// 工具总数仍为 4（不重复）
	tools, _, _ := toolRepo.List(context.Background(), repository.ToolFilter{ServerID: &srvID})
	if len(tools) != 4 {
		t.Fatalf("二次导入后工具数应仍为 4，实际 %d", len(tools))
	}
	// spec 总数仍为 4
	specs, _ := specRepo.ListByServerID(context.Background(), srvID)
	if len(specs) != 4 {
		t.Fatalf("二次导入后 spec 数应仍为 4，实际 %d", len(specs))
	}
}

func TestOpenAPIImporter_ServerNotFound(t *testing.T) {
	imp, _, _, _ := newImporter()
	_, err := imp.Import(context.Background(), 999, model.ImportOpenAPIRequest{
		Spec: json.RawMessage(petStoreSpec),
	})
	if !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("期望 ErrServerNotFound，实际 %v", err)
	}
}

func TestOpenAPIImporter_InvalidSpec(t *testing.T) {
	imp, srvRepo, _, _ := newImporter()
	srvID := seedOpenAPIServer(t, srvRepo, "http://petstore.example.com")
	_, err := imp.Import(context.Background(), srvID, model.ImportOpenAPIRequest{
		Spec: json.RawMessage(`{"not":"openapi"}`),
	})
	if !errors.Is(err, ErrInvalidOpenAPI) {
		t.Fatalf("期望 ErrInvalidOpenAPI，实际 %v", err)
	}
}

// E2E：ProxyService 调用链检测 OpenAPI 工具并翻译为实际 HTTP 请求。
func TestProxyService_CallOpenAPITool(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	specRepo := repository.NewMemoryOpenAPISpecRepository()
	svc.SetOpenAPIDeps(specRepo, NewOpenAPITranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes)))

	// 上游 API：返回 query 参数 echo
	var capturedQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		if r.Header.Get("request_id") == "" {
			t.Error("request_id 未透传到上游 OpenAPI")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pets":[{"id":1}]}`))
	}))
	defer upstream.Close()

	seedOnline(t, serverRepo, toolRepo, upstream.URL)
	// 在 tool 上挂 OpenAPI 翻译元数据
	paramsBytes, _ := json.Marshal(model.OpenAPIParams{Query: []string{"limit"}})
	if err := specRepo.Save(context.Background(), &model.OpenAPISpec{
		ToolID: 1, Method: "GET", PathTemplate: "/pets", Params: paramsBytes,
	}); err != nil {
		t.Fatal(err)
	}
	// 给 agent 授权 listPets = query_sales 复用 ID=1 的 tool
	// （stubPermissionClient 已授权 agent → query_sales/ghost，直接复用 query_sales 名）

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:    "tools/call",
		ToolName:  "query_sales",
		Arguments: map[string]interface{}{"limit": 5},
	}, "req-openapi-1")
	if err != nil {
		t.Fatalf("调用 OpenAPI 工具失败: %v", err)
	}
	if resp.IsError || len(resp.Content) != 1 {
		t.Fatalf("响应包装错误: %+v", resp)
	}
	if !strings.Contains(capturedQuery, "limit=5") {
		t.Fatalf("query 参数未拼到上游 URL: %s", capturedQuery)
	}
	if !strings.Contains(resp.Content[0]["text"].(string), "pets") {
		t.Fatalf("上游响应未透传: %+v", resp.Content[0])
	}
}

// E2E：原生 MCP 工具仍走 PostJSON 路径（specRepo 未命中时）。
func TestProxyService_NativeToolStillWorks(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	specRepo := repository.NewMemoryOpenAPISpecRepository()
	svc.SetOpenAPIDeps(specRepo, NewOpenAPITranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes)))

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 原生 MCP 协议：返回 McpToolCallResponse 结构
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"native-ok"}]}`))
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)
	// 不为 tool 1 挂 spec → 走原生路径

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		ToolName: "query_sales",
	}, "req-native-1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError || resp.Content[0]["text"] != "native-ok" {
		t.Fatalf("原生路径响应错误: %+v", resp)
	}
}
