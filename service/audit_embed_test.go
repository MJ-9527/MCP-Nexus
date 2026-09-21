package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"MCP-Nexus/model"
	"MCP-Nexus/repository"
)

// sha256Hex 测试用摘要计算。
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// waitForAudit 轮询等待异步审计落地（B8 埋点为 goroutine 异步写）。
func waitForAudit(t *testing.T, repo *repository.MemoryAuditLogRepository, want int) [](*model.AuditLog) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		logs, _, err := repo.List(context.Background(), repository.AuditLogFilter{})
		if err == nil && len(logs) >= want {
			return logs
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("等待 %d 条审计超时", want)
	return nil
}

func auditFixture(t *testing.T, downstreamURL string) (*ProxyService, *repository.MemoryAuditLogRepository) {
	t.Helper()
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, downstreamURL)
	auditRepo := repository.NewMemoryAuditLogRepository()
	svc.SetAudit(NewAuditLogService(auditRepo))
	return svc, auditRepo
}

// B8：成功调用落 success 审计，参数脱敏摘要不落原文
func TestCallToolAuditSuccess(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()
	svc, auditRepo := auditFixture(t, downstream.URL)

	args := map[string]any{"month": "2026-08", "password": "super-secret"}
	_, err := svc.CallTool(context.Background(), "agent", 1, &model.McpToolCallRequest{ToolName: "query_sales", Arguments: args}, "req-audit-1")
	if err != nil {
		t.Fatal(err)
	}
	logs := waitForAudit(t, auditRepo, 1)
	entry := logs[0]
	if entry.Status != AuditStatusSuccess {
		t.Fatalf("状态应为 success，实际 %s", entry.Status)
	}
	if entry.UserID == nil || *entry.UserID != 1 {
		t.Fatalf("审计应含 user_id=1: %+v", entry.UserID)
	}
	if entry.ToolID == nil || *entry.ToolID != 1 {
		t.Fatalf("审计应含 tool_id=1: %+v", entry.ToolID)
	}
	if entry.DurationMS < 0 {
		t.Fatalf("耗时不应为负: %d", entry.DurationMS)
	}
	if entry.ParamsDigest == "" {
		t.Fatal("参数摘要不应为空")
	}
	if len(entry.ParamsDigest) != 64 {
		t.Fatalf("应为 sha256 hex: %s", entry.ParamsDigest)
	}
	// 原文不落库：digest 不等于原文哈希之外的明文存储
	if entry.DeniedReason != "" {
		t.Fatalf("成功记录不应有拒绝原因: %s", entry.DeniedReason)
	}
}

// B8：权限拒绝落 denied 审计（含原因）
func TestCallToolAuditDenied(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")
	auditRepo := repository.NewMemoryAuditLogRepository()
	svc.SetAudit(NewAuditLogService(auditRepo))

	_, err := svc.CallTool(context.Background(), "agent", 1, &model.McpToolCallRequest{ToolName: "unauthorized_tool"}, "req-audit-2")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("期望权限拒绝: %v", err)
	}
	logs := waitForAudit(t, auditRepo, 1)
	if logs[0].Status != AuditStatusDenied {
		t.Fatalf("状态应为 denied，实际 %s", logs[0].Status)
	}
	if logs[0].DeniedReason == "" {
		t.Fatal("denied 记录应保留拒绝原因")
	}
}

// B8：其他失败（工具不存在）落 failed 审计
func TestCallToolAuditFailed(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")
	auditRepo := repository.NewMemoryAuditLogRepository()
	svc.SetAudit(NewAuditLogService(auditRepo))

	// ghost 在允许名单但 repo 中不存在 → 越过权限检查后命中"工具不存在"
	_, err := svc.CallTool(context.Background(), "agent", 1, &model.McpToolCallRequest{ToolName: "ghost"}, "req-audit-3")
	if !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("期望工具不存在: %v", err)
	}
	logs := waitForAudit(t, auditRepo, 1)
	if logs[0].Status != AuditStatusFailed {
		t.Fatalf("状态应为 failed，实际 %s", logs[0].Status)
	}
}

// B8：敏感参数脱敏后再摘要（原文哈希 ≠ 落库摘要）
func TestAuditParametersSanitizedBeforeDigest(t *testing.T) {
	auditRepo := repository.NewMemoryAuditLogRepository()
	svc := NewAuditLogService(auditRepo)
	entry, err := svc.Record(context.Background(), model.CreateAuditLogRequest{
		RequestID:  "req-san",
		Status:     AuditStatusSuccess,
		Parameters: map[string]any{"password": "super-secret", "month": "2026-08"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 若未脱敏，digest = sha256('{"month":"2026-08","password":"super-secret"}')
	// 脱敏后 digest = sha256('{"month":"2026-08","password":"***"}')
	rawHash := sha256Hex([]byte(`{"month":"2026-08","password":"***"}`))
	if entry.ParamsDigest != rawHash {
		t.Fatalf("摘要应基于脱敏后参数: got=%s want=%s", entry.ParamsDigest, rawHash)
	}
}

// B14：被 RBAC 拒绝的调用也要记录工具名与角色 —— 此时还没查到 toolID，
// 若不落 ToolName，分析存储里就无法按工具维度统计拒绝情况。
func TestCallToolAuditFillsToolNameAndRoleWhenDenied(t *testing.T) {
	svc, toolRepo, serverRepo := newFixture(t)
	seedOnline(t, serverRepo, toolRepo, "http://unused")
	auditRepo := repository.NewMemoryAuditLogRepository()
	svc.SetAudit(NewAuditLogService(auditRepo))

	_, err := svc.CallTool(context.Background(), "anonymous", 42, &model.McpToolCallRequest{ToolName: "query_sales"}, "req-tn-1")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("期望权限拒绝: %v", err)
	}
	entry := waitForAudit(t, auditRepo, 1)[0]
	if entry.ToolName != "query_sales" {
		t.Fatalf("被拒记录也应带工具名，实际 %q", entry.ToolName)
	}
	if entry.CallerRole != "anonymous" {
		t.Fatalf("应记录调用者角色，实际 %q", entry.CallerRole)
	}
	if entry.ToolID != nil {
		t.Fatalf("被拒时尚未解析到工具，tool_id 应为空，实际 %v", *entry.ToolID)
	}
}

// B14：成功调用的审计同样带工具名与角色。
func TestCallToolAuditFillsToolNameAndRoleOnSuccess(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer downstream.Close()

	svc, auditRepo := auditFixture(t, downstream.URL)
	if _, err := svc.CallTool(context.Background(), "agent", 1,
		&model.McpToolCallRequest{ToolName: "query_sales", Arguments: map[string]any{"month": "2026-08"}}, "req-tn-2"); err != nil {
		t.Fatal(err)
	}
	entry := waitForAudit(t, auditRepo, 1)[0]
	if entry.ToolName != "query_sales" || entry.CallerRole != "agent" {
		t.Fatalf("成功记录应带工具名与角色: tool=%q role=%q", entry.ToolName, entry.CallerRole)
	}
	if entry.ToolID == nil || *entry.ToolID != 1 {
		t.Fatalf("成功记录应带 tool_id=1: %+v", entry.ToolID)
	}
}
