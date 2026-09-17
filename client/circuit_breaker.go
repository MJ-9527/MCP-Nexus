package client

import (
	"errors"
	"sync"
	"time"
)

// 熔断器（B13）。
//
// 设计目标：上游服务连续故障时快速失败，避免网关在已知的故障上游上堆积
// 请求、占用连接与 goroutine，造成级联雪崩。熔断器按 endpoint 维度独立
// 维护，互不影响。
//
// 状态机：
//   - Closed：正常放行；记录连续失败，达到阈值 → Open
//   - Open：直接拒绝；cooldown 到期 → HalfOpen
//   - HalfOpen：仅允许 1 个探测请求；成功 → Closed，失败 → Open
//
// 计数规则：仅统计「连续失败」；成功立即清零。这样间歇性故障不会误触
// 熔断，只有持续不可用才会触发保护。

// CircuitState 熔断状态。
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // 正常放行
	CircuitOpen                         // 熔断中，拒绝所有请求
	CircuitHalfOpen                     // 半开，仅允许探测
)

// ErrCircuitOpen 熔断器开启，请求被直接拒绝（不会发送到下游）。
// service 层应映射为 ErrServerUnavailable（503 SERVER_UNAVAILABLE）。
var ErrCircuitOpen = errors.New("circuit open")

// CircuitBreaker 单个 endpoint 的熔断器。
// 并发安全：所有方法内部加锁。
type CircuitBreaker struct {
	mu            sync.Mutex
	state         CircuitState
	failureCount  int           // Closed 状态下累计的连续失败数
	threshold     int           // 连续失败阈值
	cooldown      time.Duration // Open 持续时间
	openedAt      time.Time     // 进入 Open 的时刻（用于判断 cooldown 是否到期）
	probeInflight bool          // HalfOpen 状态下是否已有探测请求在途
}

// NewCircuitBreaker 创建熔断器。
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreaker{
		state:     CircuitClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// Allow 检查是否放行请求。返回 nil 表示放行；返回 ErrCircuitOpen 表示拒绝。
// HalfOpen 状态下仅允许 1 个探测请求；其他请求被拒绝（不消耗下游资源）。
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		// cooldown 到期则进入 HalfOpen，尝试探测
		if time.Since(cb.openedAt) >= cb.cooldown {
			cb.state = CircuitHalfOpen
			cb.probeInflight = true
			return nil
		}
		return ErrCircuitOpen
	case CircuitHalfOpen:
		// 已有探测请求在途：拒绝其他请求，避免半开瞬间放行大量流量
		if cb.probeInflight {
			return ErrCircuitOpen
		}
		cb.probeInflight = true
		return nil
	}
	return nil
}

// RecordSuccess 记录成功。Closed 状态清零失败计数；HalfOpen 状态恢复为 Closed。
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.probeInflight = false
	cb.state = CircuitClosed
}

// RecordFailure 记录失败。
//   - Closed：失败数 +1，达阈值 → Open
//   - HalfOpen：探测失败 → Open（重置 cooldown 计时）
//   - Open：忽略（已有冷却计时，不重复刷新）
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case CircuitClosed:
		cb.failureCount++
		if cb.failureCount >= cb.threshold {
			cb.state = CircuitOpen
			cb.openedAt = time.Now()
			cb.probeInflight = false
		}
	case CircuitHalfOpen:
		// 探测失败，立即恢复 Open 重新冷却
		cb.state = CircuitOpen
		cb.openedAt = time.Now()
		cb.probeInflight = false
	case CircuitOpen:
		// 已经 Open：不刷新 openedAt，避免持续失败无限延后半开时机
	}
}

// State 返回当前状态（仅供测试与可观测性查询，非并发互斥用途）。
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// FailureCount 返回当前连续失败计数（仅供测试与可观测性）。
func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failureCount
}

// CircuitBreakerRegistry 按 endpoint 维度管理熔断器，避免并发创建。
type CircuitBreakerRegistry struct {
	mu        sync.RWMutex
	breakers  map[string]*CircuitBreaker
	threshold int
	cooldown  time.Duration
}

// NewCircuitBreakerRegistry 构造熔断器注册表。
func NewCircuitBreakerRegistry(threshold int, cooldown time.Duration) *CircuitBreakerRegistry {
	if threshold <= 0 {
		threshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &CircuitBreakerRegistry{
		breakers:  make(map[string]*CircuitBreaker),
		threshold: threshold,
		cooldown:  cooldown,
	}
}

// Get 返回指定 endpoint 的熔断器；不存在则按统一参数新建。
func (r *CircuitBreakerRegistry) Get(endpoint string) *CircuitBreaker {
	r.mu.RLock()
	if cb, ok := r.breakers[endpoint]; ok {
		r.mu.RUnlock()
		return cb
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// 双检：可能在升级写锁期间被其他 goroutine 创建
	if cb, ok := r.breakers[endpoint]; ok {
		return cb
	}
	cb := NewCircuitBreaker(r.threshold, r.cooldown)
	r.breakers[endpoint] = cb
	return cb
}
