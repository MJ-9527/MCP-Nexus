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

func newSkillsImporter() (*SkillsImporter, *repository.MemoryServerRepository, *repository.MemoryToolRepository, *repository.MemorySkillsSpecRepository) {
	srvRepo := repository.NewMemoryServerRepository()
	toolRepo := repository.NewMemoryToolRepository()
	specRepo := repository.NewMemorySkillsSpecRepository()
	return NewSkillsImporter(srvRepo, toolRepo, specRepo), srvRepo, toolRepo, specRepo
}

func seedSkillsServer(t *testing.T, srvRepo *repository.MemoryServerRepository, endpoint string) int64 {
	t.Helper()
	srv := &model.MCPServer{ID: 1, Name: "skills-svc", Endpoint: endpoint, Version: "1.0.0", Status: "active", HealthStatus: "online"}
	if err := srvRepo.Create(context.Background(), srv); err != nil {
		t.Fatal(err)
	}
	return srv.ID
}

// sampleSkillsTools 构造两个 Skills 工具定义：一个 GET 带 path+query，一个 POST 带 body。
func sampleSkillsTools() []model.SkillsToolDef {
	return []model.SkillsToolDef{
		{
			Name:         "search_kb",
			Description:  "搜索知识库",
			InputSchema:  json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
			Version:      "1.2.0",
			Endpoint:     "http://skills.example.com",
			Method:       "GET",
			PathTemplate: "/kb/{kbId}/search",
			Params: model.SkillsParams{
				Path:  []string{"kbId"},
				Query: []string{"q"},
			},
		},
		{
			Name:         "create_doc",
			Description:  "创建文档",
			InputSchema:  json.RawMessage(`{"type":"object","properties":{"body":{"type":"object"}}}`),
			Version:      "1.0.0",
			Endpoint:     "http://skills.example.com",
			Method:       "POST",
			PathTemplate: "/docs",
			Params: model.SkillsParams{
				Body: "body",
			},
		},
	}
}

func TestSkillsImporter_CreateTools(t *testing.T) {
	imp, srvRepo, toolRepo, specRepo := newSkillsImporter()
	srvID := seedSkillsServer(t, srvRepo, "http://skills.example.com")

	result, err := imp.Import(context.Background(), srvID, model.ImportSkillsRequest{
		Tools:     sampleSkillsTools(),
		Published: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 {
		t.Fatalf("期望导入 2 个，实际 %d", result.Total)
	}
	// 工具应入库且 category=skills
	tools, _, _ := toolRepo.List(context.Background(), repository.ToolFilter{ServerID: &srvID})
	if len(tools) != 2 {
		t.Fatalf("工具入库数量 %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Category != "skills" {
			t.Errorf("工具 %s category 应为 skills，实际 %s", tool.Name, tool.Category)
		}
		if !tool.Published {
			t.Errorf("工具 %s 应已发布", tool.Name)
		}
	}
	// spec 应入库
	specs, _ := specRepo.ListByServerID(context.Background(), srvID)
	if len(specs) != 2 {
		t.Fatalf("spec 入库数量 %d", len(specs))
	}
}

func TestSkillsImporter_Idempotent(t *testing.T) {
	imp, srvRepo, toolRepo, specRepo := newSkillsImporter()
	srvID := seedSkillsServer(t, srvRepo, "http://skills.example.com")

	req := model.ImportSkillsRequest{Tools: sampleSkillsTools()}
	// 第一次导入
	r1, err := imp.Import(context.Background(), srvID, req)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Total != 2 {
		t.Fatalf("首次导入应 2 个，实际 %d", r1.Total)
	}
	// 第二次导入：应复用工具（action=updated），不重复创建
	r2, err := imp.Import(context.Background(), srvID, req)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Total != 2 {
		t.Fatalf("二次导入应 2 个，实际 %d", r2.Total)
	}
	for _, e := range r2.Imported {
		if e.Action != "updated" {
			t.Errorf("二次导入工具 %s 应为 updated，实际 %s", e.Name, e.Action)
		}
	}
	// 工具总数仍为 2（不重复）
	tools, _, _ := toolRepo.List(context.Background(), repository.ToolFilter{ServerID: &srvID})
	if len(tools) != 2 {
		t.Fatalf("二次导入后工具数应仍为 2，实际 %d", len(tools))
	}
	// spec 总数仍为 2
	specs, _ := specRepo.ListByServerID(context.Background(), srvID)
	if len(specs) != 2 {
		t.Fatalf("二次导入后 spec 数应仍为 2，实际 %d", len(specs))
	}
}

func TestSkillsImporter_ServerNotFound(t *testing.T) {
	imp, _, _, _ := newSkillsImporter()
	_, err := imp.Import(context.Background(), 999, model.ImportSkillsRequest{
		Tools: sampleSkillsTools(),
	})
	if !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("期望 ErrServerNotFound，实际 %v", err)
	}
}

func TestSkillsImporter_InvalidSpec(t *testing.T) {
	imp, srvRepo, _, _ := newSkillsImporter()
	srvID := seedSkillsServer(t, srvRepo, "http://skills.example.com")
	// 空 name
	_, err := imp.Import(context.Background(), srvID, model.ImportSkillsRequest{
		Tools: []model.SkillsToolDef{
			{Name: "", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	})
	if !errors.Is(err, ErrInvalidSkills) {
		t.Fatalf("期望 ErrInvalidSkills，实际 %v", err)
	}
	// 非法 InputSchema
	_, err = imp.Import(context.Background(), srvID, model.ImportSkillsRequest{
		Tools: []model.SkillsToolDef{
			{Name: "bad", InputSchema: json.RawMessage(`{not-json`)},
		},
	})
	if !errors.Is(err, ErrInvalidSkills) {
		t.Fatalf("期望 ErrInvalidSkills，实际 %v", err)
	}
	// 请求体内重名
	_, err = imp.Import(context.Background(), srvID, model.ImportSkillsRequest{
		Tools: []model.SkillsToolDef{
			{Name: "dup", InputSchema: json.RawMessage(`{"type":"object"}`)},
			{Name: "dup", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	})
	if !errors.Is(err, ErrInvalidSkills) {
		t.Fatalf("期望 ErrInvalidSkills，实际 %v", err)
	}
}

// 默认 path_template：未指定时按 /tools/{name}/call 落地，与原生 MCP 兼容。
func TestSkillsImporter_DefaultPathTemplate(t *testing.T) {
	imp, srvRepo, _, specRepo := newSkillsImporter()
	srvID := seedSkillsServer(t, srvRepo, "http://skills.example.com")

	_, err := imp.Import(context.Background(), srvID, model.ImportSkillsRequest{
		Tools: []model.SkillsToolDef{
			{
				Name:        "echo",
				InputSchema: json.RawMessage(`{"type":"object"}`),
				// 不指定 Method/PathTemplate/Params
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := specRepo.FindByToolID(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Method != "POST" {
		t.Errorf("默认 method 应为 POST，实际 %s", spec.Method)
	}
	if spec.PathTemplate != "/tools/echo/call" {
		t.Errorf("默认 path_template 应为 /tools/echo/call，实际 %s", spec.PathTemplate)
	}
	if spec.Endpoint != "" {
		t.Errorf("未指定 endpoint 应为空串，实际 %s", spec.Endpoint)
	}
}

// E2E：ProxyService 调用链检测 Skills 工具并翻译为实际 HTTP 请求（spec.Endpoint 覆盖 server.Endpoint）。
func TestProxyService_CallSkillsTool(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	skillsSpecRepo := repository.NewMemorySkillsSpecRepository()
	svc.SetSkillsDeps(skillsSpecRepo, NewSkillsTranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes)))

	var capturedPath, capturedQuery, capturedMethod string
	// 上游 Skills 服务（不同于 server.Endpoint）
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedMethod = r.Method
		if r.Header.Get("request_id") == "" {
			t.Error("request_id 未透传到上游 Skills")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"skills-ok"}`))
	}))
	defer upstream.Close()

	// server.Endpoint 用一个不可达地址，证明 spec.Endpoint 覆盖生效
	seedOnline(t, serverRepo, toolRepo, "http://unreachable.example.com")
	// 在 tool id=1 上挂 Skills 翻译元数据，强制走 Skills 路径
	paramsBytes, _ := json.Marshal(model.SkillsParams{Path: []string{"kbId"}, Query: []string{"q"}})
	if err := skillsSpecRepo.Save(context.Background(), &model.SkillsSpec{
		ToolID:       1,
		Endpoint:     upstream.URL,
		Method:       "GET",
		PathTemplate: "/kb/{kbId}/search",
		Params:       paramsBytes,
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"kbId": "123",
			"q":    "hello",
		},
	}, "req-skills-1")
	if err != nil {
		t.Fatalf("调用 Skills 工具失败: %v", err)
	}
	if resp.IsError || len(resp.Content) != 1 {
		t.Fatalf("响应包装错误: %+v", resp)
	}
	if capturedMethod != "GET" || capturedPath != "/kb/123/search" || !strings.Contains(capturedQuery, "q=hello") {
		t.Fatalf("上游请求不符 method=%s path=%s query=%s", capturedMethod, capturedPath, capturedQuery)
	}
	if !strings.Contains(resp.Content[0]["text"].(string), "skills-ok") {
		t.Fatalf("上游响应未透传: %+v", resp.Content[0])
	}
}

// E2E：Skills 工具的 ListTools 与原生 MCP 工具一致（category=skills 不影响发现）。
// 通过 SkillsImporter 完整导入一个 Skills 工具，验证 ListTools 可发现、CallTool 可调用。
func TestProxyService_SkillsToolDiscoveredAndCallable(t *testing.T) {
	// 复用 newFixture 的 serverRepo/toolRepo/stubPermissionClient（agent 已授权 query_sales）
	svc, toolRepo, serverRepo := newFixture(t)
	skillsSpecRepo := repository.NewMemorySkillsSpecRepository()
	svc.SetSkillsDeps(skillsSpecRepo, NewSkillsTranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes)))

	// 上游 Skills 服务（不同于 server.Endpoint，证明 spec.Endpoint 覆盖生效）
	var capturedPath, capturedQuery, capturedMethod string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedMethod = r.Method
		if r.Header.Get("request_id") == "" {
			t.Error("request_id 未透传到上游 Skills")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"skills-ok"}`))
	}))
	defer upstream.Close()

	// 1. 创建一个 Server（status=active, health=online）
	srv := &model.MCPServer{
		ID: 1, Name: "skills-svc",
		Endpoint:     "http://unreachable.example.com", // 故意不可达，证明 spec.Endpoint 覆盖
		Version:      "1.0.0",
		Status:       "active",
		HealthStatus: "online",
	}
	if err := serverRepo.Create(context.Background(), srv); err != nil {
		t.Fatal(err)
	}

	// 2. 用 SkillsImporter 导入一个名为 query_sales 的 Skills 工具（复用 stubPermissionClient 已授权）
	imp := NewSkillsImporter(serverRepo, toolRepo, skillsSpecRepo)
	_, err := imp.Import(context.Background(), srv.ID, model.ImportSkillsRequest{
		Tools: []model.SkillsToolDef{
			{
				Name:         "query_sales",
				Description:  "Skills 适配的查询销售工具",
				InputSchema:  json.RawMessage(`{"type":"object","properties":{"kbId":{"type":"string"},"q":{"type":"string"}},"required":["kbId","q"]}`),
				Version:      "1.0.0",
				Endpoint:     upstream.URL,
				Method:       "GET",
				PathTemplate: "/kb/{kbId}/search",
				Params: model.SkillsParams{
					Path:  []string{"kbId"},
					Query: []string{"q"},
				},
			},
		},
		Published: true, // 立即发布，便于发现
	})
	if err != nil {
		t.Fatalf("Skills 导入失败: %v", err)
	}

	// 3. ListTools 应能发现 query_sales（category=skills 不影响发现链路）
	listResp, err := svc.ListTools(context.Background(), "agent", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(listResp.Tools) != 1 || listResp.Tools[0].Name != "query_sales" {
		t.Fatalf("Skills 工具应可被发现，实际 %+v", listResp.Tools)
	}

	// 4. CallTool 应翻译为 HTTP 请求发送到上游 Skills 服务
	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		Method:   "tools/call",
		ToolName: "query_sales",
		Arguments: map[string]interface{}{
			"kbId": "123",
			"q":    "hello",
		},
	}, "req-skills-e2e")
	if err != nil {
		t.Fatalf("调用 Skills 工具失败: %v", err)
	}
	if resp.IsError || len(resp.Content) != 1 {
		t.Fatalf("响应包装错误: %+v", resp)
	}
	if capturedMethod != "GET" || capturedPath != "/kb/123/search" || !strings.Contains(capturedQuery, "q=hello") {
		t.Fatalf("上游请求不符 method=%s path=%s query=%s", capturedMethod, capturedPath, capturedQuery)
	}
	if !strings.Contains(resp.Content[0]["text"].(string), "skills-ok") {
		t.Fatalf("上游响应未透传: %+v", resp.Content[0])
	}
}

// E2E：原生 MCP 工具在注入 Skills deps 后仍走 PostJSON 路径（skillsSpecRepo 未命中时）。
func TestProxyService_NativeToolStillWorksAfterSkills(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	skillsSpecRepo := repository.NewMemorySkillsSpecRepository()
	svc.SetSkillsDeps(skillsSpecRepo, NewSkillsTranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes)))

	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"native-ok"}]}`))
	}))
	defer downstream.Close()
	seedOnline(t, serverRepo, toolRepo, downstream.URL)
	// 不为 tool 1 挂 skills spec → 走原生路径

	resp, err := svc.CallTool(context.Background(), "agent", 0, &model.McpToolCallRequest{
		ToolName: "query_sales",
	}, "req-native-after-skills")
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError || resp.Content[0]["text"] != "native-ok" {
		t.Fatalf("原生路径响应错误: %+v", resp)
	}
}
