package client

import (
	"testing"
	"time"
)

// B14：验证 CircuitBreakerRegistry.List 暴露所有 endpoint 状态。
func TestCircuitBreakerRegistry_List(t *testing.T) {
	reg := NewCircuitBreakerRegistry(3, 50*time.Millisecond)

	// 触发 endpoint A 熔断：3 次失败
	cbA := reg.Get("http://a")
	cbA.RecordFailure()
	cbA.RecordFailure()
	cbA.RecordFailure()
	// endpoint B 仅 1 次失败：仍 Closed
	cbB := reg.Get("http://b")
	cbB.RecordFailure()

	items := reg.List()
	if len(items) != 2 {
		t.Fatalf("应返回 2 个 endpoint，实际 %d", len(items))
	}
	// 按 endpoint 名升序：a 在前 b 在后
	if items[0].Endpoint != "http://a" || items[0].State != CircuitOpen {
		t.Fatalf("a 应 Open，实际 %+v", items[0])
	}
	if items[0].FailureCount != 3 {
		t.Fatalf("a 失败计数应为 3，实际 %d", items[0].FailureCount)
	}
	if items[1].Endpoint != "http://b" || items[1].State != CircuitClosed {
		t.Fatalf("b 应 Closed，实际 %+v", items[1])
	}
}

// B14：验证未注册任何熔断器时 List 返回空切片（非 nil）。
func TestCircuitBreakerRegistry_ListEmpty(t *testing.T) {
	reg := NewCircuitBreakerRegistry(3, 50*time.Millisecond)
	items := reg.List()
	if items == nil {
		t.Fatal("应返回非 nil 空切片")
	}
	if len(items) != 0 {
		t.Fatalf("空注册表应返回 0 项，实际 %d", len(items))
	}
}
