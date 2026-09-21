package repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"MCP-Nexus/model"
)

// captureClickHouse 起一个假的 ClickHouse HTTP 端点，记录收到的请求。
type captureClickHouse struct {
	mu       sync.Mutex
	requests int
	method   string
	path     string
	query    map[string]string
	body     string
	headers  http.Header
	status   int
	response string
}

func newCaptureClickHouse(status int, response string) *captureClickHouse {
	return &captureClickHouse{status: status, response: response}
}

func (c *captureClickHouse) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	c.requests++
	c.method = r.Method
	c.path = r.URL.Path
	c.query = map[string]string{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			c.query[k] = v[0]
		}
	}
	c.headers = r.Header.Clone()
	raw, _ := io.ReadAll(r.Body)
	c.body = string(raw)
	c.mu.Unlock()
	w.WriteHeader(c.status)
	_, _ = w.Write([]byte(c.response))
}

func (c *captureClickHouse) snapshot() (int, string, map[string]string, string, http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests, c.method, c.query, c.body, c.headers
}

func int64Ptr(v int64) *int64 { return &v }

// B14：批量写入应发送 JSONEachRow，字段名与列名对齐，空值序列化为 null。
func TestClickHouseSinkWriteBatchSendsJSONEachRow(t *testing.T) {
	fake := newCaptureClickHouse(http.StatusOK, "")
	srv := httptest.NewServer(fake)
	defer srv.Close()

	sink, err := NewClickHouseAuditSink(srv.URL, "mcp_analytics", "audit_logs", "", "", 2*time.Second)
	if err != nil {
		t.Fatalf("构造 sink 失败: %v", err)
	}

	createdAt := time.Date(2026, 9, 18, 10, 30, 0, 123_000_000, time.UTC)
	logs := []*model.AuditLog{
		{
			RequestID: "req-1", UserID: int64Ptr(7), ToolID: int64Ptr(1), ServerID: int64Ptr(2),
			ToolName: "query_sales", CallerRole: "agent", DurationMS: 42, Status: "success",
			HTTPStatus: 200, ParamsSummary: "sha256:abc", ParamsSensitiveMasked: true,
			ParamsDigest: "abc", CostEstimate: 0.5, CreatedAt: createdAt,
		},
		{
			RequestID: "req-2", ToolName: "ghost", CallerRole: "agent", DurationMS: 3,
			Status: "denied", DeniedReason: "not authorized",
		},
	}
	if err := sink.WriteBatch(context.Background(), logs); err != nil {
		t.Fatalf("WriteBatch 失败: %v", err)
	}

	requests, method, query, body, headers := fake.snapshot()
	if requests != 1 {
		t.Fatalf("应只发一次请求，实际 %d", requests)
	}
	if method != http.MethodPost {
		t.Fatalf("应为 POST，实际 %s", method)
	}
	if query["database"] != "mcp_analytics" {
		t.Fatalf("database 参数应为 mcp_analytics，实际 %q", query["database"])
	}
	if query["query"] != "INSERT INTO mcp_analytics.audit_logs FORMAT JSONEachRow" {
		t.Fatalf("query 参数不符: %q", query["query"])
	}
	if ct := headers.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("Content-Type 应为 application/x-ndjson，实际 %q", ct)
	}

	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("应有 2 行 NDJSON，实际 %d：%q", len(lines), body)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("首行不是合法 JSON: %v", err)
	}
	for _, key := range []string{
		"request_id", "user_id", "tool_id", "server_id", "tool_name", "caller_role",
		"duration_ms", "status", "http_status", "denied_reason", "reject_reason",
		"params_summary", "params_sensitive_masked", "params_digest", "cost_estimate", "created_at",
	} {
		if _, ok := first[key]; !ok {
			t.Errorf("缺少列 %q（列名必须与 ClickHouse 表一致）", key)
		}
	}
	if first["created_at"] != "2026-09-18 10:30:00.123" {
		t.Fatalf("created_at 应为 ClickHouse 规范格式，实际 %v", first["created_at"])
	}
	if first["params_sensitive_masked"] != true {
		t.Fatalf("params_sensitive_masked 应为布尔 true，实际 %v", first["params_sensitive_masked"])
	}

	// 第二行：可空字段应为 null，避免 ClickHouse Nullable 列写入失败
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("次行不是合法 JSON: %v", err)
	}
	for _, key := range []string{"user_id", "tool_id", "server_id"} {
		if v, ok := second[key]; !ok || v != nil {
			t.Errorf("%s 应为 null，实际 %v", key, v)
		}
	}
}

// B14：非 2xx 应返回错误并带上上游错误片段，便于排查。
func TestClickHouseSinkWriteBatchSurfacesUpstreamError(t *testing.T) {
	fake := newCaptureClickHouse(http.StatusInternalServerError, "Code: 60. DB::Exception: Table mcp_analytics.audit_logs doesn't exist")
	srv := httptest.NewServer(fake)
	defer srv.Close()

	sink, err := NewClickHouseAuditSink(srv.URL, "mcp_analytics", "audit_logs", "", "", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = sink.WriteBatch(context.Background(), []*model.AuditLog{{RequestID: "r", Status: "success"}})
	if err == nil {
		t.Fatal("非 2xx 应返回错误")
	}
	if !strings.Contains(err.Error(), "doesn't exist") {
		t.Fatalf("错误应包含上游片段，实际 %v", err)
	}
}

// B14：空批次不应发起请求。
func TestClickHouseSinkWriteBatchEmptyIsNoop(t *testing.T) {
	fake := newCaptureClickHouse(http.StatusOK, "")
	srv := httptest.NewServer(fake)
	defer srv.Close()

	sink, err := NewClickHouseAuditSink(srv.URL, "mcp_analytics", "audit_logs", "", "", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteBatch(context.Background(), nil); err != nil {
		t.Fatalf("空批次不应报错: %v", err)
	}
	if err := sink.WriteBatch(context.Background(), []*model.AuditLog{}); err != nil {
		t.Fatalf("空批次不应报错: %v", err)
	}
	if requests, _, _, _, _ := fake.snapshot(); requests != 0 {
		t.Fatalf("空批次不应发起请求，实际 %d 次", requests)
	}
}

// B14：配置了账号密码时应带 ClickHouse 认证头。
func TestClickHouseSinkSendsAuthHeaders(t *testing.T) {
	fake := newCaptureClickHouse(http.StatusOK, "")
	srv := httptest.NewServer(fake)
	defer srv.Close()

	sink, err := NewClickHouseAuditSink(srv.URL, "mcp_analytics", "audit_logs", "audit_writer", "s3cret", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteBatch(context.Background(), []*model.AuditLog{{RequestID: "r", Status: "success"}}); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, headers := fake.snapshot()
	if headers.Get("X-ClickHouse-User") != "audit_writer" || headers.Get("X-ClickHouse-Key") != "s3cret" {
		t.Fatalf("认证头缺失: user=%q key=%q", headers.Get("X-ClickHouse-User"), headers.Get("X-ClickHouse-Key"))
	}
}

// B14：库名/表名非法应直接拒绝，避免拼接出意外 SQL。
func TestNewClickHouseAuditSinkRejectsInvalidIdentifier(t *testing.T) {
	if _, err := NewClickHouseAuditSink("localhost:8123", "bad;drop", "audit_logs", "", "", time.Second); err == nil {
		t.Fatal("非法库名应被拒绝")
	}
	if _, err := NewClickHouseAuditSink("localhost:8123", "mcp_analytics", "audit logs", "", "", time.Second); err == nil {
		t.Fatal("非法表名应被拒绝")
	}
	if _, err := NewClickHouseAuditSink("   ", "mcp_analytics", "audit_logs", "", "", time.Second); err == nil {
		t.Fatal("空地址应被拒绝")
	}
}

// B14：地址未带 scheme 时应自动补 http://，并去掉尾部斜杠；空库名/表名走默认值。
func TestNewClickHouseAuditSinkNormalizesAddr(t *testing.T) {
	sink, err := NewClickHouseAuditSink("clickhouse:8123/", "", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if sink.ch.baseURL != "http://clickhouse:8123" {
		t.Fatalf("baseURL 归一化错误: %q", sink.ch.baseURL)
	}
	if sink.ch.database != "mcp_analytics" || sink.table != DefaultClickHouseTable {
		t.Fatalf("默认库表名不符: db=%q table=%q", sink.ch.database, sink.table)
	}
}
