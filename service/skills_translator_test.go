package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"MCP-Nexus/client"
	"MCP-Nexus/model"
)

func newSkillsTranslator() *SkillsTranslator {
	return NewSkillsTranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes))
}

// buildSkillsSpec 测试辅助：构造 SkillsSpec，params 序列化为 JSON。
func buildSkillsSpec(t *testing.T, endpoint, method, path string, params model.SkillsParams) *model.SkillsSpec {
	t.Helper()
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return &model.SkillsSpec{ToolID: 1, Endpoint: endpoint, Method: method, PathTemplate: path, Params: b}
}

// spec.Endpoint 非空时，baseURL 应取自 spec 而非 server.Endpoint。
func TestSkillsTranslator_EndpointOverride(t *testing.T) {
	var capturedHost string
	// 上游服务：监听到 spec.Endpoint 的主机名
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHost = r.Host
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// server.Endpoint 用一个不可达地址，证明 spec.Endpoint 覆盖生效
	spec := buildSkillsSpec(t, srv.URL, "GET", "/kb/search", model.SkillsParams{Query: []string{"q"}})
	tr := newSkillsTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: "http://unreachable.example.com", Status: "active", HealthStatus: "online"},
		map[string]interface{}{"q": "hello"}, "req-sk-1")
	if err != nil {
		t.Fatalf("调用 Skills 工具失败: %v", err)
	}
	if resp.IsError {
		t.Fatalf("响应不应标记错误: %+v", resp)
	}
	if !strings.Contains(capturedHost, strings.TrimPrefix(srv.URL, "http://")) {
		t.Fatalf("未使用 spec.Endpoint，实际 host=%s", capturedHost)
	}
}

// spec.Endpoint 空时，baseURL fallback 到 server.Endpoint。
func TestSkillsTranslator_EndpointFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	spec := buildSkillsSpec(t, "", "GET", "/kb/search", model.SkillsParams{Query: []string{"q"}})
	tr := newSkillsTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"q": "hello"}, "req-sk-2")
	if err != nil {
		t.Fatalf("fallback 调用失败: %v", err)
	}
}

func TestSkillsTranslator_GetWithPathAndQuery(t *testing.T) {
	var capturedPath, capturedQuery, capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedMethod = r.Method
		_, _ = w.Write([]byte(`{"doc":{"id":"123","title":"hello"}}`))
	}))
	defer srv.Close()

	spec := buildSkillsSpec(t, srv.URL, "GET", "/kb/{kbId}/search", model.SkillsParams{
		Path:  []string{"kbId"},
		Query: []string{"q"},
	})
	tr := newSkillsTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"kbId": "123", "q": "hello"}, "req-sk-3")
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "GET" || capturedPath != "/kb/123/search" || !strings.Contains(capturedQuery, "q=hello") {
		t.Fatalf("上游请求不符 method=%s path=%s query=%s", capturedMethod, capturedPath, capturedQuery)
	}
	if resp.IsError || len(resp.Content) != 1 {
		t.Fatalf("响应包装错误: %+v", resp)
	}
	if !strings.Contains(resp.Content[0]["text"].(string), "hello") {
		t.Fatalf("响应内容未透传: %+v", resp.Content[0])
	}
}

func TestSkillsTranslator_PostWithBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = b
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":99}`))
	}))
	defer srv.Close()

	spec := buildSkillsSpec(t, srv.URL, "POST", "/docs", model.SkillsParams{Body: "body"})
	tr := newSkillsTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"body": map[string]interface{}{"title": "T1"}}, "req-sk-4")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capturedBody), `"title":"T1"`) {
		t.Fatalf("请求体未透传: %s", capturedBody)
	}
	if resp.IsError {
		t.Fatalf("201 不应视为错误: %+v", resp)
	}
}

func TestSkillsTranslator_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	spec := buildSkillsSpec(t, srv.URL, "GET", "/kb/{kbId}", model.SkillsParams{Path: []string{"kbId"}})
	tr := newSkillsTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"kbId": "999"}, "req-sk-5")
	if err != nil {
		t.Fatalf("404 不应产生传输错误，实际 %v", err)
	}
	if !resp.IsError {
		t.Fatalf("404 应标记 IsError=true")
	}
}

func TestSkillsTranslator_MissingPathParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("上游不应被调用")
	}))
	defer srv.Close()

	spec := buildSkillsSpec(t, srv.URL, "GET", "/kb/{kbId}/search", model.SkillsParams{Path: []string{"kbId"}})
	tr := newSkillsTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"q": "hi"}, "req-sk-6") // 缺 kbId
	if err == nil || !strings.Contains(err.Error(), "kbId") {
		t.Fatalf("缺 path 参数应报错，实际 %v", err)
	}
}

// 默认 path_template /tools/{name}/call 且 params.Path 为空时，
// 应自动用 arguments 中同名字段填充 {占位符}。
func TestSkillsTranslator_DefaultPathPlaceholderFill(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// 模拟 SkillsImporter 默认落地的 spec：path_template=/tools/echo/call，params 为空
	spec := buildSkillsSpec(t, srv.URL, "POST", "/tools/echo/call", model.SkillsParams{})
	tr := newSkillsTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"msg": "hi"}, "req-sk-7")
	if err != nil {
		t.Fatalf("默认路径调用失败: %v", err)
	}
	if capturedPath != "/tools/echo/call" {
		t.Fatalf("默认路径未透传: %s", capturedPath)
	}
}

func TestSkillsTranslator_UpstreamUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	spec := buildSkillsSpec(t, srv.URL, "GET", "/kb", model.SkillsParams{})
	tr := newSkillsTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{}, "req-sk-8")
	if err == nil {
		t.Fatal("不可达上游应报错")
	}
}

func TestBuildSkillsRequest_PathURLEncode(t *testing.T) {
	// path 参数值含 / 应被 URL 编码，避免路径注入
	url, _, err := buildSkillsRequest("http://skills.example.com", "/kb/{kbId}", model.SkillsParams{Path: []string{"kbId"}}, map[string]interface{}{"kbId": "a/b"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(url, "/kb/a/b") {
		t.Fatalf("path 参数未做 URL 编码: %s", url)
	}
	if !strings.Contains(url, "/kb/a%2Fb") {
		t.Fatalf("path 参数编码结果不符: %s", url)
	}
}
