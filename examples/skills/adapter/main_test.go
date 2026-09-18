package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRecorder(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handleTool(w, req)
	return w
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handleHealth(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %v", body["status"])
	}
}

func TestCalculateVAT(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/calculate_vat/call", `{"amount":100,"rate":0.13}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	res := body["result"].(map[string]any)
	if res["total"] != float64(113) {
		t.Errorf("total = %v, want 113", res["total"])
	}
}

func TestFormatPhone(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/format_phone/call", `{"phone":"13812345678"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	res := body["result"].(map[string]any)
	if res["formatted"] != "138-1234-5678" {
		t.Errorf("formatted = %v", res["formatted"])
	}
}

func TestToolNotFound(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/nope/call", `{}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestInvalidJSON(t *testing.T) {
	w := newRecorder(http.MethodPost, "/tools/calculate_vat/call", `{amount`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
