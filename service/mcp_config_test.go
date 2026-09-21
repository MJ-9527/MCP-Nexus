package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, downstream.URL)

	cfgSvc := NewMcpConfigService(svc, "24h0m0s")
	resp, err := cfgSvc.Generate(context.Background(), "agent", 0, "http://192.168.1.5:8080")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Protocol != "custom-rest" {
		t.Fatalf("protocol = %s", resp.Protocol)
	}
	if resp.Endpoint != "http://192.168.1.5:8080/mcp" {
		t.Fatalf("endpoint = %s", resp.Endpoint)
	}
	if resp.Auth.Type != "bearer" || resp.Auth.TokenTTL != "24h0m0s" {
		t.Fatalf("auth 指引错误: %+v", resp.Auth)
	}
	// 操作说明与端点拼接
	if len(resp.Operations) != 2 {
		t.Fatalf("应包含 2 个操作说明: %d", len(resp.Operations))
	}
	if resp.Operations[0].Name != "list_tools" || resp.Operations[1].Name != "call_tool" {
		t.Fatalf("操作说明名称错误: %v", resp.Operations)
	}
	if !strings.Contains(resp.Operations[1].RequestExample, "/mcp/tools/query_sales/call") {
		t.Fatalf("调用示例应含工具路径: %s", resp.Operations[1].RequestExample)
	}
	// 工具清单与 ListTools 可见性一致
	names := make([]string, 0, len(resp.Tools))
	for _, tool := range resp.Tools {
		names = append(names, tool.Name)
	}
	if len(names) != 1 || names[0] != "query_sales" {
		t.Fatalf("工具清单应仅含可见的 query_sales: %v", names)
	}
}

func TestGenerateConfigEmptyTools(t *testing.T) {
	svc, _, _ := newFixture(t)
	// 无任何授权 → 清单为空数组而非 null
	cfgSvc := NewMcpConfigService(svc, "24h")
	resp, err := cfgSvc.Generate(context.Background(), "nobody", 0, "http://x:8080")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Tools == nil {
		t.Fatal("Tools 应为空数组而非 null")
	}
	if len(resp.Tools) != 0 {
		t.Fatalf("应无工具: %v", resp.Tools)
	}
}
