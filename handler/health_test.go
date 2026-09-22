package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"MCP-Nexus/client"
	"github.com/gin-gonic/gin"
)

func TestHealthBasic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Health(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "OK" {
		t.Errorf("code = %v, want OK", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want ok", data["status"])
	}
}

func TestHealthHandlerWithDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	}))
	defer server.Close()

	h := NewHealthHandler(nil, client.NewHealthClient(5*time.Second), []DownstreamService{
		{Name: "demo", Endpoint: server.URL},
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	h.Health(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data := body["data"].(map[string]any)
	deps := data["dependencies"].(map[string]any)
	demo := deps["demo"].(map[string]any)
	if demo["status"] != "healthy" {
		t.Errorf("demo status = %v, want healthy", demo["status"])
	}
	if data["status"] != "degraded" {
		t.Errorf("overall status = %v, want degraded (postgres unknown)", data["status"])
	}
}
