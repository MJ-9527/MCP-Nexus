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

func newTranslator() *OpenAPITranslator {
	return NewOpenAPITranslator(client.NewMCPClient(CallTimeout, client.DefaultMaxBodyBytes))
}

// buildSpec 测试辅助：构造 OpenAPISpec，params 序列化为 JSON。
func buildSpec(t *testing.T, method, path string, params model.OpenAPIParams) *model.OpenAPISpec {
	t.Helper()
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return &model.OpenAPISpec{ToolID: 1, Method: method, PathTemplate: path, Params: b}
}

func TestTranslator_GetWithPathParam(t *testing.T) {
	var capturedPath, capturedQuery, capturedMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pet":{"id":"123","name":"Fido"}}`))
	}))
	defer srv.Close()

	spec := buildSpec(t, "GET", "/pets/{petId}", model.OpenAPIParams{Path: []string{"petId"}})
	tr := newTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL, Status: "active", HealthStatus: "online"},
		map[string]interface{}{"petId": "123"}, "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "GET" || capturedPath != "/pets/123" || capturedQuery != "" {
		t.Fatalf("上游请求不符 method=%s path=%s query=%s", capturedMethod, capturedPath, capturedQuery)
	}
	if resp.IsError || len(resp.Content) != 1 {
		t.Fatalf("响应包装错误: %+v", resp)
	}
	if !strings.Contains(resp.Content[0]["text"].(string), "Fido") {
		t.Fatalf("响应内容未透传: %+v", resp.Content[0])
	}
}

func TestTranslator_GetWithQueryParam(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"id":1},{"id":2}]`))
	}))
	defer srv.Close()

	spec := buildSpec(t, "GET", "/pets", model.OpenAPIParams{Query: []string{"limit", "status"}})
	tr := newTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"limit": 10, "status": "available"}, "req-2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedQuery, "limit=10") || !strings.Contains(capturedQuery, "status=available") {
		t.Fatalf("query 参数未拼到 URL: %s", capturedQuery)
	}
}

func TestTranslator_PostWithBody(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = b
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":99}`))
	}))
	defer srv.Close()

	spec := buildSpec(t, "POST", "/pets", model.OpenAPIParams{Body: "body"})
	tr := newTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"body": map[string]interface{}{"name": "Rex"}}, "req-3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capturedBody), `"name":"Rex"`) {
		t.Fatalf("请求体未透传: %s", capturedBody)
	}
	if resp.IsError {
		t.Fatalf("201 不应视为错误: %+v", resp)
	}
}

func TestTranslator_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"pet not found"}`))
	}))
	defer srv.Close()

	spec := buildSpec(t, "GET", "/pets/{petId}", model.OpenAPIParams{Path: []string{"petId"}})
	tr := newTranslator()
	resp, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{"petId": "999"}, "req-4")
	if err != nil {
		t.Fatalf("404 不应产生传输错误，实际 %v", err)
	}
	if !resp.IsError {
		t.Fatalf("404 应标记 IsError=true")
	}
	if !strings.Contains(resp.Content[0]["text"].(string), "pet not found") {
		t.Fatalf("错误响应体未透传: %+v", resp.Content[0])
	}
}

func TestTranslator_MissingPathParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("上游不应被调用")
	}))
	defer srv.Close()

	spec := buildSpec(t, "GET", "/pets/{petId}", model.OpenAPIParams{Path: []string{"petId"}})
	tr := newTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{}, "req-5") // 缺 petId
	if err == nil || !strings.Contains(err.Error(), "petId") {
		t.Fatalf("缺 path 参数应报错，实际 %v", err)
	}
}

func TestTranslator_UpstreamUnreachable(t *testing.T) {
	// 用一个已关闭的 server 模拟不可达
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	spec := buildSpec(t, "GET", "/pets", model.OpenAPIParams{})
	tr := newTranslator()
	_, err := tr.Translate(context.Background(), spec,
		&model.MCPServer{Endpoint: srv.URL},
		map[string]interface{}{}, "req-6")
	if err == nil {
		t.Fatal("不可达上游应报错")
	}
	// 应映射为 ErrServerUnavailable 或 ErrUpstreamError
	if !strings.Contains(err.Error(), "server unavailable") && !strings.Contains(err.Error(), "upstream") {
		t.Fatalf("错误映射不符预期: %v", err)
	}
}

func TestBuildOpenAPIRequest_PathURLEncode(t *testing.T) {
	// path 参数值含 / 应被 URL 编码，避免路径注入
	url, _, err := buildOpenAPIRequest("http://api.example.com", "/pets/{petId}", model.OpenAPIParams{Path: []string{"petId"}}, map[string]interface{}{"petId": "a/b"})
	if err != nil {
		t.Fatal(err)
	}
	// / 应编码为 %2F，不应出现在 path 段中
	if strings.Contains(url, "/pets/a/b") {
		t.Fatalf("path 参数未做 URL 编码: %s", url)
	}
	if !strings.Contains(url, "/pets/a%2Fb") {
		t.Fatalf("path 参数编码结果不符: %s", url)
	}
}
