package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRecorder(method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := setupRouter(nil)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newRecorderWithBase(method, path, body, base string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := setupRouter(nil, base)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newRecorderWithKey(method, path, body, apiKey string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := setupRouter(nil)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHealth(t *testing.T) {
	w := newRecorder(http.MethodGet, "/health", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response 不是合法 JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["service"] != "demo-service" {
		t.Errorf("service = %v, want demo-service", body["service"])
	}
	if body["health"] != true {
		t.Errorf("health = %v, want true", body["health"])
	}
}

func TestQuerySales(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/query_sales/call", `{"month":"2026-08"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response 不是合法 JSON: %v", err)
	}
	// JSON 数字反序列化后是 float64
	if body["month"] != "2026-08" || body["total_sales"] != float64(128000) || body["region"] != "华东" {
		t.Errorf("响应字段不符: %v", body)
	}
	if body["tool"] != "query_sales" {
		t.Errorf("tool = %v, want query_sales", body["tool"])
	}
}

func TestQuerySalesInvalidJSON(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/query_sales/call", `{"month":`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestQuerySalesEmptyBody(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/query_sales/call", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestQueryCustomerDBUnavailable(t *testing.T) {
	// 未配置数据库（db == nil）时应返回 503，而不是 panic 或 500。
	w := newRecorder(http.MethodPost, "/tools/query_customer/call", `{"region":"华东"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestQueryCustomerInvalidJSON(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/query_customer/call", `{"region":`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestReadFileSuccess(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "config.json"), []byte(`{"app":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := newRecorderWithBase(http.MethodPost, "/tools/read_file/call", `{"path":"config.json"}`, base)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response 不是合法 JSON: %v", err)
	}
	if body["content"] != `{"app":"demo"}` {
		t.Errorf("content = %v, want %q", body["content"], `{"app":"demo"}`)
	}
}

func TestReadFileTraversal(t *testing.T) {
	base := t.TempDir()
	w := newRecorderWithBase(http.MethodPost, "/tools/read_file/call", `{"path":"../../etc/passwd"}`, base)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestReadFileNotFound(t *testing.T) {
	base := t.TempDir()
	w := newRecorderWithBase(http.MethodPost, "/tools/read_file/call", `{"path":"nope.txt"}`, base)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestReadFileMissingPath(t *testing.T) {
	base := t.TempDir()
	w := newRecorderWithBase(http.MethodPost, "/tools/read_file/call", `{}`, base)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestFetchURLDomainNotAllowed(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/fetch_url/call", `{"url":"http://evil.com/x"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestFetchURLInvalidURL(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/fetch_url/call", `{"url":"not-a-url"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestDeleteCustomerUnauthorized(t *testing.T) {
	// 无 API Key：应返回 401，且不会执行下游 DELETE。
	w := newRecorder(http.MethodPost, "/tools/delete_customer/call", `{"id":1}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestDeleteCustomerWrongKey(t *testing.T) {
	w := newRecorderWithKey(http.MethodPost, "/tools/delete_customer/call", `{"id":1}`, "wrong-key")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestDeleteCustomerValidKeyDBUnavailable(t *testing.T) {
	// 正确 API Key 通过鉴权后进入处理器；db == nil 时返回 503，证明已越过鉴权门槛。
	w := newRecorderWithKey(http.MethodPost, "/tools/delete_customer/call", `{"id":1}`, "demo-api-key")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestDeleteCustomerInvalidJSON(t *testing.T) {
	w := newRecorderWithKey(http.MethodPost, "/tools/delete_customer/call", `{"id":`, "demo-api-key")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
