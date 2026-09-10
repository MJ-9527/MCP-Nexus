package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRecorder(method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := setupRouter()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
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
