package repository

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"MCP-Nexus/model"
)

// 对真实 ClickHouse 的集成验证：确认 JSONEachRow 报文与表结构（Nullable / Bool /
// DateTime64）完全兼容——这类问题用假服务器测不出来，必须打真实实例。
//
// 默认跳过；设置 CLICKHOUSE_HTTP_ADDR 后启用，例如：
//
//	CLICKHOUSE_HTTP_ADDR=127.0.0.1:8123 go test ./repository/ -run ClickHouseSinkIntegration -v
//
// 前置条件：目标库已应用 db/clickhouse/init/001_create_audit_logs.sql。
func TestClickHouseSinkIntegrationWriteAndReadBack(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("CLICKHOUSE_HTTP_ADDR"))
	if addr == "" {
		t.Skip("未设置 CLICKHOUSE_HTTP_ADDR，跳过 ClickHouse 集成测试")
	}
	db := os.Getenv("CLICKHOUSE_DB")
	if db == "" {
		db = "mcp_analytics"
	}

	sink, err := NewClickHouseAuditSink(addr, db, "audit_logs",
		os.Getenv("CLICKHOUSE_USER"), os.Getenv("CLICKHOUSE_PASSWORD"), 10*time.Second)
	if err != nil {
		t.Fatalf("构造 sink 失败: %v", err)
	}

	requestID := fmt.Sprintf("it-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	rows := []*model.AuditLog{
		{
			RequestID: requestID, UserID: int64Ptr(1), ToolID: int64Ptr(2), ServerID: int64Ptr(3),
			ToolName: "it_tool", CallerRole: "agent", DurationMS: 42, Status: "success",
			HTTPStatus: 200, ParamsSummary: "sha256:deadbeef", ParamsSensitiveMasked: true,
			ParamsDigest: "deadbeef", CostEstimate: 0.25, CreatedAt: now,
		},
		{
			// 可空字段全部留空，验证 Nullable 列写入
			RequestID: requestID + "-2", ToolName: "it_tool", CallerRole: "agent",
			DurationMS: 7, Status: "denied", DeniedReason: "not authorized", CreatedAt: now,
		},
	}

	if err := sink.WriteBatch(context.Background(), rows); err != nil {
		t.Fatalf("写入真实 ClickHouse 失败: %v", err)
	}

	query := fmt.Sprintf(
		"SELECT count(), countIf(user_id IS NULL), countIf(params_sensitive_masked), sum(duration_ms) FROM %s.audit_logs WHERE request_id IN ('%s', '%s')",
		db, requestID, requestID+"-2")
	got := chQuery(t, addr, db, query)
	// 2 条记录；第二条可空字段为 NULL；仅第一条标记脱敏；耗时合计 49
	if got != "2\t1\t1\t49" {
		t.Fatalf("读回校验失败：期望 2\\t1\\t1\\t49，实际 %q", got)
	}

	// 清理本次测试数据，避免污染分析表
	chQuery(t, addr, db, fmt.Sprintf(
		"ALTER TABLE %s.audit_logs DELETE WHERE request_id IN ('%s', '%s')", db, requestID, requestID+"-2"))
}

// chQuery 通过 HTTP 接口执行查询并返回 TSV 文本。
// 统一用 POST：ClickHouse 对 HTTP 的 GET 请求按只读处理，清理类语句必须用 POST。
func chQuery(t *testing.T, addr, db, query string) string {
	t.Helper()
	base := addr
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	values := url.Values{}
	values.Set("database", db)
	values.Set("query", query)

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/?"+values.Encode(), nil)
	if err != nil {
		t.Fatalf("构造 ClickHouse 请求失败: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("查询 ClickHouse 失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("查询 ClickHouse 返回 %d: %s", resp.StatusCode, string(body))
	}
	return strings.TrimSpace(string(body))
}
