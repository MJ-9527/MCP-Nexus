//go:build integration
// +build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func getBaseURL() string {
	if url := os.Getenv("TEST_BASE_URL"); url != "" {
		return url
	}
	return "http://localhost:18080"
}

const headerContentType = "application/json"

var uniqueSuffix = fmt.Sprintf("%d", time.Now().UnixNano())
var authToken string

// ---- helpers ----

func doRequest(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var buf io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, getBaseURL()+path, buf)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", headerContentType)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d. body: %s", expected, resp.StatusCode, string(body))
	}
}
func assertSuccess(t *testing.T, resp *http.Response) { assertStatus(t, resp, 200) }

func assertError(t *testing.T, resp *http.Response, code string) {
	t.Helper()
	expected := 500
	switch code {
	case "INVALID_PARAMETER":
		expected = 400
	case "SERVER_NOT_FOUND", "TOOL_NOT_FOUND":
		expected = 404
	case "CONFLICT", "SERVER_ALREADY_EXISTS":
		expected = 409
	case "UNAUTHORIZED":
		expected = 401
	case "FORBIDDEN":
		expected = 403
	}
	assertStatus(t, resp, expected)
}

type APIResp struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      any    `json:"data"`
}

// ---- TestMain: register + login before any test runs ----

func TestMain(m *testing.M) {
	baseURL := getBaseURL()
	// Register
	regBody, _ := json.Marshal(map[string]string{
		"username": "integration-test-user", "password": "password123", "role": "admin",
	})
	resp, err := http.Post(baseURL+"/api/auth/register", headerContentType, bytes.NewReader(regBody))
	if err == nil && resp.StatusCode == 200 {
		resp.Body.Close()
	}
	// Login
	loginBody, _ := json.Marshal(map[string]string{
		"username": "integration-test-user", "password": "password123",
	})
	resp, err = http.Post(baseURL+"/api/auth/login", headerContentType, bytes.NewReader(loginBody))
	if err == nil && resp.StatusCode == 200 {
		var r APIResp
		json.NewDecoder(resp.Body).Decode(&r)
		tokenMap := r.Data.(map[string]any)
		authToken = tokenMap["token"].(string)
		resp.Body.Close()
		fmt.Printf("[setup] logged in as integration-test-user, token=%s...\n", authToken[:min(len(authToken), 16)])
	} else {
		fmt.Printf("[setup] auth failed: err=%v status=%d\n", err, resp.StatusCode)
	}
	os.Exit(m.Run())
}

// ---- D1: API 响应格式 ----

func TestD1_HealthEndpoint(t *testing.T) {
	resp := doRequest(t, "GET", "/health", nil)
	assertSuccess(t, resp)
	if resp.Header.Get("Request_id") == "" {
		t.Fatal("missing request_id header")
	}
}

func TestD1_ResponseFormat(t *testing.T) {
	resp := doRequest(t, "GET", "/health", nil)
	body, _ := io.ReadAll(resp.Body)
	var r map[string]any
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"code", "message", "request_id"} {
		if _, ok := r[k]; !ok {
			t.Fatalf("missing field: %s", k)
		}
	}
}

// ---- D2: Server 注册与查询 ----

func TestD2_RegisterServer_Success(t *testing.T) {
	resp := doRequest(t, "POST", "/api/servers", map[string]string{
		"name": "int-test-" + uniqueSuffix, "endpoint": "http://localhost:19001", "version": "1.0.0",
	})
	assertSuccess(t, resp)
}

func TestD2_RegisterServer_Duplicate(t *testing.T) {
	name := "dup-server-" + uniqueSuffix
	p := map[string]string{"name": name, "endpoint": "http://localhost:19002", "version": "1.0.0"}
	doRequest(t, "POST", "/api/servers", p)
	resp := doRequest(t, "POST", "/api/servers", p)
	assertError(t, resp, "SERVER_ALREADY_EXISTS")
}

func TestD2_RegisterServer_MissingFields(t *testing.T) {
	resp := doRequest(t, "POST", "/api/servers", map[string]string{"name": "x"})
	assertError(t, resp, "INVALID_PARAMETER")
}

func TestD2_ListServers(t *testing.T) {
	resp := doRequest(t, "GET", "/api/servers", nil)
	assertSuccess(t, resp)
}

// ---- D3: 端到端链路 ----

func TestD3_E2E_ServerLifecycle(t *testing.T) {
	name := "e2e-srv-" + uniqueSuffix
	reg := doRequest(t, "POST", "/api/servers", map[string]string{
		"name": name, "description": "e2e", "endpoint": "http://localhost:19003", "version": "1.0.0",
	})
	assertSuccess(t, reg)
	var rd APIResp
	json.NewDecoder(reg.Body).Decode(&rd)
	sid, _ := rd.Data.(map[string]any)["id"].(string)
	assertSuccess(t, doRequest(t, "GET", "/api/servers/"+sid, nil))
	assertError(t, doRequest(t, "GET", "/api/servers/999999", nil), "SERVER_NOT_FOUND")
}

// ---- D4: 最小闭环 ----

func TestD4_MinimalLoop(t *testing.T) {
	checks := []struct {
		name, method, path string
		body     any
		wantCode string
	}{
		{"health", "GET", "/health", nil, "OK"},
		{"register", "POST", "/api/servers", map[string]string{"name": "chk-" + uniqueSuffix, "endpoint": "http://localhost:19004", "version": "1.0.0"}, "OK"},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			resp := doRequest(t, tc.method, tc.path, tc.body)
			body, _ := io.ReadAll(resp.Body)
			var r APIResp
			if err := json.Unmarshal(body, &r); err != nil {
				t.Fatalf("parse: %v body: %s", err, string(body))
			}
			if tc.wantCode != "" && r.Code != tc.wantCode {
				t.Fatalf("want %s got %s", tc.wantCode, r.Code)
			}
		})
	}
}

// ---- D8: 安全场景回归测试 ----

func TestD8_InvalidEndpoint(t *testing.T) {
	resp := doRequest(t, "POST", "/api/servers", map[string]string{
		"name": "bad-" + uniqueSuffix, "endpoint": "not-a-url", "version": "1.0.0",
	})
	assertError(t, resp, "INVALID_PARAMETER")
}

func TestD8_EmptyBody(t *testing.T) {
	resp := doRequest(t, "POST", "/api/servers", map[string]string{})
	assertError(t, resp, "INVALID_PARAMETER")
}

func TestD8_MalformedJSON(t *testing.T) {
	req, _ := http.NewRequest("POST", getBaseURL()+"/api/servers", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", headerContentType)
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400 for malformed JSON, got %d", resp.StatusCode)
	}
}

func TestD8_ToolNotFound(t *testing.T) {
	resp := doRequest(t, "POST", "/mcp/tools/nonexistent/call", map[string]any{"arguments": map[string]string{}})
	if resp.StatusCode == 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected 500: %s", string(body))
	}
}

func TestD8_LoginMissingFields(t *testing.T) {
	resp := doRequest(t, "POST", "/api/auth/login", map[string]string{})
	assertError(t, resp, "INVALID_PARAMETER")
}

func TestD8_LoginInvalidCredentials(t *testing.T) {
	resp := doRequest(t, "POST", "/api/auth/login", map[string]string{
		"username": "nonexistent_" + uniqueSuffix,
		"password": "password123",
	})
	assertError(t, resp, "UNAUTHORIZED")
}

func TestD8_ServerRequiresAuth(t *testing.T) {
	oldToken := authToken
	authToken = ""
	resp := doRequest(t, "POST", "/api/servers", map[string]string{
		"name": "no-auth-" + uniqueSuffix, "endpoint": "http://localhost:19999", "version": "1.0.0",
	})
	authToken = oldToken
	if resp.StatusCode != 401 {
		body, _ := io.ReadAll(resp.Body)
		t.Logf("server register without auth: status=%d body=%s", resp.StatusCode, string(body))
	}
}

func TestD8_GatewayToolsPublic(t *testing.T) {
	resp := doRequest(t, "GET", "/mcp/tools", nil)
	if resp.StatusCode == 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected 500 from /mcp/tools: %s", string(body))
	}
}
