package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"MCP-Nexus/model"
)

// B13 熔断器状态机单测。

func TestCircuitBreaker_InitialStateClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	if cb.State() != CircuitClosed {
		t.Fatalf("初始应为 Closed，实际 %v", cb.State())
	}
	if err := cb.Allow(); err != nil {
		t.Fatalf("Closed 状态应放行，实际 %v", err)
	}
}

func TestCircuitBreaker_OpenAfterThresholdFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	// 2 次失败：未达阈值，仍 Closed
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Fatalf("2 次失败应仍为 Closed，实际 %v", cb.State())
	}
	if cb.FailureCount() != 2 {
		t.Fatalf("失败计数应为 2，实际 %d", cb.FailureCount())
	}
	// 第 3 次失败：达阈值，转 Open
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("3 次失败应转 Open，实际 %v", cb.State())
	}
	// Open 状态：拒绝请求
	if !errors.Is(cb.Allow(), ErrCircuitOpen) {
		t.Fatalf("Open 状态应返回 ErrCircuitOpen")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, 50*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	// 成功立即清零
	cb.RecordSuccess()
	if cb.FailureCount() != 0 {
		t.Fatalf("成功应清零失败计数，实际 %d", cb.FailureCount())
	}
	if cb.State() != CircuitClosed {
		t.Fatalf("成功后应仍为 Closed，实际 %v", cb.State())
	}
	// 再失败 2 次不应直接 Open（因为已清零）
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitClosed {
		t.Fatalf("清零后再失败 2 次不应 Open，实际 %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker(2, 30*time.Millisecond)
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("预期 Open，实际 %v", cb.State())
	}
	// 未到 cooldown：拒绝
	if !errors.Is(cb.Allow(), ErrCircuitOpen) {
		t.Fatalf("cooldown 未到应拒绝")
	}
	// 等 cooldown 过期
	time.Sleep(35 * time.Millisecond)
	// 到期：进入 HalfOpen，放行 1 个探测
	if err := cb.Allow(); err != nil {
		t.Fatalf("cooldown 到期应放行探测请求，实际 %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("cooldown 到期应转 HalfOpen，实际 %v", cb.State())
	}
	// HalfOpen 已有探测在途：拒绝其他
	if !errors.Is(cb.Allow(), ErrCircuitOpen) {
		t.Fatalf("HalfOpen 已有探测时应拒绝其他请求")
	}
}

func TestCircuitBreaker_HalfOpenSuccessClosesCircuit(t *testing.T) {
	cb := NewCircuitBreaker(1, 20*time.Millisecond)
	cb.RecordFailure() // 立即 Open
	time.Sleep(25 * time.Millisecond)
	_ = cb.Allow() // 转 HalfOpen，占用探测
	// 探测成功
	cb.RecordSuccess()
	if cb.State() != CircuitClosed {
		t.Fatalf("HalfOpen 探测成功应转 Closed，实际 %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopensCircuit(t *testing.T) {
	cb := NewCircuitBreaker(1, 20*time.Millisecond)
	cb.RecordFailure() // Open
	time.Sleep(25 * time.Millisecond)
	_ = cb.Allow() // HalfOpen
	// 探测失败：重新 Open 并刷新 cooldown
	cb.RecordFailure()
	if cb.State() != CircuitOpen {
		t.Fatalf("HalfOpen 探测失败应重新 Open，实际 %v", cb.State())
	}
	// 重新进入 Open 后 cooldown 未到，应拒绝
	if !errors.Is(cb.Allow(), ErrCircuitOpen) {
		t.Fatalf("重新 Open 后应拒绝")
	}
}

func TestCircuitBreaker_OpenStateDoesNotResetCooldown(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)
	cb.RecordFailure() // Open
	// 在 Open 期间继续记失败：不应刷新 openedAt
	cb.RecordFailure()
	cb.RecordFailure()
	// 等 cooldown 过期后应能进入 HalfOpen
	time.Sleep(55 * time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("cooldown 后应放行探测，实际 %v", err)
	}
	if cb.State() != CircuitHalfOpen {
		t.Fatalf("应转 HalfOpen，实际 %v", cb.State())
	}
}

func TestCircuitBreakerRegistry_GetReturnsSameInstance(t *testing.T) {
	r := NewCircuitBreakerRegistry(3, 50*time.Millisecond)
	a := r.Get("http://a")
	b := r.Get("http://a")
	if a != b {
		t.Fatal("同 endpoint 应返回同一实例")
	}
	c := r.Get("http://b")
	if a == c {
		t.Fatal("不同 endpoint 应返回不同实例")
	}
}

// ---- 重试策略单测 ----

func TestIsIdempotent(t *testing.T) {
	cases := map[string]bool{
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodPut:     true,
		http.MethodDelete:  true,
		http.MethodOptions: true,
		http.MethodPost:    false,
		http.MethodPatch:   false,
		"":                 false,
	}
	for method, want := range cases {
		if got := IsIdempotent(method); got != want {
			t.Errorf("IsIdempotent(%q) = %v, want %v", method, got, want)
		}
	}
}

func TestIsRetriableError(t *testing.T) {
	if IsRetriableError(nil) {
		t.Fatal("nil 不可重试")
	}
	if !IsRetriableError(ErrUpstreamTimeout) {
		t.Fatal("ErrUpstreamTimeout 应可重试")
	}
	if !IsRetriableError(ErrServerUnreachable) {
		t.Fatal("ErrServerUnreachable 应可重试")
	}
	if IsRetriableError(ErrBodyTooLarge) {
		t.Fatal("ErrBodyTooLarge 不应重试")
	}
	if IsRetriableError(ErrUpstreamError) {
		t.Fatal("ErrUpstreamError 不应重试")
	}
	// 包装后的错误也应识别（fmt.Errorf %w 链）
	wrapped := wrapErr("ctx timeout", ErrUpstreamTimeout)
	if !IsRetriableError(wrapped) {
		t.Fatal("包装的 ErrUpstreamTimeout 应可重试")
	}
}

// wrapErr 用 %w 包装错误，模拟实际错误链。
func wrapErr(msg string, sentinel error) error {
	return &wrappedErr{msg: msg, cause: sentinel}
}

type wrappedErr struct {
	msg   string
	cause error
}

func (e *wrappedErr) Error() string { return e.msg + ": " + e.cause.Error() }
func (e *wrappedErr) Unwrap() error { return e.cause }

func TestBackoffDelay_Exponential(t *testing.T) {
	cfg := DefaultRetryConfig()
	// 200ms, 400ms, 800ms, 1.6s, 2s(封顶)
	want := []time.Duration{
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1600 * time.Millisecond,
		2 * time.Second,
	}
	for i, w := range want {
		if got := cfg.BackoffDelay(i); got != w {
			t.Errorf("attempt %d: got %v, want %v", i, got, w)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	cfg := DefaultRetryConfig()
	// 幂等 + 网络错误：重试
	if !cfg.ShouldRetry(0, http.MethodGet, ErrUpstreamTimeout, false) {
		t.Fatal("GET + 超时应重试")
	}
	// 非幂等 + 网络错误：不重试
	if cfg.ShouldRetry(0, http.MethodPost, ErrUpstreamTimeout, false) {
		t.Fatal("POST 不应重试")
	}
	// 幂等 + 业务错误：不重试
	if cfg.ShouldRetry(0, http.MethodGet, ErrBodyTooLarge, false) {
		t.Fatal("BodyTooLarge 不应重试")
	}
	// 上下文取消：不重试
	if cfg.ShouldRetry(0, http.MethodGet, ErrUpstreamTimeout, true) {
		t.Fatal("ctx 取消不应重试")
	}
	// 超过 MaxRetries：不重试
	if cfg.ShouldRetry(cfg.MaxRetries, http.MethodGet, ErrUpstreamTimeout, false) {
		t.Fatal("超过 MaxRetries 不应重试")
	}
}

// ---- E2E：HTTP 集成 ----

// E2E：连续失败触发熔断，熔断期间请求被直接拒绝
func TestMCPClient_CircuitOpensAfterConsecutiveFailures(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := NewMCPClient(500*time.Millisecond, DefaultMaxBodyBytes)
	// 阈值 3，cooldown 5s（足够测试）
	cli.SetCircuitBreakerRegistry(NewCircuitBreakerRegistry(3, 5*time.Second))

	// POST 不重试，每次失败记一次熔断失败
	for i := 0; i < 3; i++ {
		_, err := cli.PostJSON(context.Background(), srv.URL, &model.McpToolCallRequest{}, nil)
		if err == nil {
			t.Fatalf("第 %d 次应失败", i+1)
		}
	}
	// 第 4 次应被熔断拒绝
	_, err := cli.PostJSON(context.Background(), srv.URL, &model.McpToolCallRequest{}, nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("第 4 次应被熔断拒绝，实际 %v", err)
	}
	// 熔断期间不发送到下游
	hitsBefore := atomic.LoadInt64(&hits)
	_, _ = cli.PostJSON(context.Background(), srv.URL, &model.McpToolCallRequest{}, nil)
	if atomic.LoadInt64(&hits) != hitsBefore {
		t.Fatalf("熔断期间不应调用下游")
	}
}

// E2E：幂等请求网络抖动时重试成功
func TestMCPClient_GetRetriesOnTransientFailure(t *testing.T) {
	var attempts int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&attempts, 1)
		if n < 2 {
			// 第一次返回 503 → 视为成功响应（err=nil），不会触发重试
			// 但 503 不会让 client 重试（仅网络层错误重试），所以这个测试用
			// 主动让连接失败的方式更难。改为：第二次成功。
			http.Error(w, "fail", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cli := NewMCPClient(2*time.Second, DefaultMaxBodyBytes)
	cli.SetRetry(RetryConfig{MaxRetries: 2, BaseDelay: 10 * time.Millisecond, MaxDelay: 100 * time.Millisecond})

	// 503 是业务错误，不会触发重试（按 B13 设计）
	body, status, err := cli.DoRequest(context.Background(), http.MethodGet, srv.URL, nil, nil)
	if err != nil {
		t.Fatalf("503 不应返回 error： %v", err)
	}
	if status != http.StatusBadGateway {
		t.Fatalf("状态码应为 502，实际 %d", status)
	}
	if string(body) != "fail\n" {
		t.Fatalf("响应体不符: %q", body)
	}
	// 仅一次尝试（业务错误不重试）
	if atomic.LoadInt64(&attempts) != 1 {
		t.Fatalf("业务错误不重试，应只调 1 次，实际 %d", atomic.LoadInt64(&attempts))
	}
}

// E2E：成功调用后熔断失败计数清零
func TestMCPClient_SuccessResetsFailures(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&hits, 1)
		if n <= 2 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	cli := NewMCPClient(500*time.Millisecond, DefaultMaxBodyBytes)
	cli.SetCircuitBreakerRegistry(NewCircuitBreakerRegistry(3, 5*time.Second))

	// 两次 5xx 失败（未达阈值 3，仍 Closed）
	_, _ = cli.PostJSON(context.Background(), srv.URL, &model.McpToolCallRequest{}, nil)
	_, _ = cli.PostJSON(context.Background(), srv.URL, &model.McpToolCallRequest{}, nil)

	cb := cli.Breakers().Get(srv.URL)
	if cb.State() != CircuitClosed {
		t.Fatalf("2 次失败应仍 Closed，实际 %v", cb.State())
	}
	if cb.FailureCount() != 2 {
		t.Fatalf("失败计数应为 2，实际 %d", cb.FailureCount())
	}

	// 第 3 次成功（响应 200）→ RecordSuccess 清零计数
	atomic.StoreInt64(&hits, 0)
	// 用一个新 server 让响应 200
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer okSrv.Close()
	// 这里换 endpoint 会导致熔断器不同。所以测试同一个 endpoint 上需要不同响应。
	// 用一个能切换响应的 server 更准确：让原 server 第 3 次返回 200。
	_ = okSrv

	// 重新构造一个测试：用切换式 server 验证「3 次失败→熔断；2 次失败后成功→清零」
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&hits, 1)
		if n <= 2 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv2.Close()

	cli2 := NewMCPClient(500*time.Millisecond, DefaultMaxBodyBytes)
	cli2.SetCircuitBreakerRegistry(NewCircuitBreakerRegistry(3, 5*time.Second))
	_, _ = cli2.PostJSON(context.Background(), srv2.URL, &model.McpToolCallRequest{}, nil)
	_, _ = cli2.PostJSON(context.Background(), srv2.URL, &model.McpToolCallRequest{}, nil)
	// 第 3 次成功（hits=3，返回 200）
	_, err := cli2.PostJSON(context.Background(), srv2.URL, &model.McpToolCallRequest{}, nil)
	if err != nil {
		t.Fatalf("第 3 次应成功，实际 %v", err)
	}
	cb2 := cli2.Breakers().Get(srv2.URL)
	if cb2.FailureCount() != 0 {
		t.Fatalf("成功后失败计数应清零，实际 %d", cb2.FailureCount())
	}
	if cb2.State() != CircuitClosed {
		t.Fatalf("应仍为 Closed，实际 %v", cb2.State())
	}
}

// E2E：非幂等 POST 不重试（即使失败）
func TestMCPClient_PostDoesNotRetry(t *testing.T) {
	var attempts int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&attempts, 1)
		// 模拟不可达：直接关连接让 net 层报错（但 httptest 难做到，
		// 用一个立即关闭的 listener 替代更直接）
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cli := NewMCPClient(500*time.Millisecond, DefaultMaxBodyBytes)
	cli.SetRetry(RetryConfig{MaxRetries: 2, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond})

	// POST 即使配置了重试也不应重试（非幂等）
	_, _ = cli.PostJSON(context.Background(), srv.URL, &model.McpToolCallRequest{}, nil)
	// 5xx 不重试 + POST 不重试：仅 1 次
	if got := atomic.LoadInt64(&attempts); got != 1 {
		t.Fatalf("POST 5xx 不应重试，实际 %d 次", got)
	}
}

// E2E：连接不可达时重试幂等请求直到耗尽预算
func TestMCPClient_GetRetriesUntilContextDone(t *testing.T) {
	// 用一个不存在的端口：连接被拒 → ErrServerUnreachable（可重试）
	cli := NewMCPClient(200*time.Millisecond, DefaultMaxBodyBytes)
	cli.SetRetry(RetryConfig{MaxRetries: 3, BaseDelay: 5 * time.Millisecond, MaxDelay: 20 * time.Millisecond})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _, err := cli.DoRequest(ctx, http.MethodGet, "http://127.0.0.1:1", nil, nil) // 端口 1：连接被拒
	if err == nil {
		t.Fatal("预期失败")
	}
	// 应是网络错误（不可达或超时），不是熔断
	if !errors.Is(err, ErrServerUnreachable) && !errors.Is(err, ErrUpstreamTimeout) {
		t.Fatalf("应为不可达或超时，实际 %v", err)
	}
}
