package client

import (
	"errors"
	"net/http"
	"time"
)

// 重试策略（B13）。
//
// 设计目标：对网络瞬时故障（连接超时/请求超时/连接被拒）做有限重试，
// 避免一次性抖动导致调用失败。仅在请求语义上「幂等」的方法才重试，
// 避免对非幂等的 POST 重放造成业务侧重复写入。
//
// 重试边界：
//  - 仅重试网络层错误（ErrUpstreamTimeout / ErrServerUnreachable）
//  - HTTP 业务错误（4xx/5xx 响应）不重试：重试也无法改变结果
//  - 非幂等方法不重试：POST/PUT 副作用未确认前重放可能造成重复副作用
//  - 响应体超限（ErrBodyTooLarge）不重试：上游已成功返回，问题在响应规模
//
// 退避策略：指数退避 base * 2^attempt，避免重试风暴冲击已故障的上游。

// RetryConfig 重试配置。
type RetryConfig struct {
	MaxRetries int           // 最大重试次数（不含首次尝试），默认 2
	BaseDelay  time.Duration // 首次重试前的基础等待，默认 200ms
	MaxDelay   time.Duration // 单次退避上限，默认 2s
}

// DefaultRetryConfig 默认重试配置：最多重试 2 次（共 3 次尝试）。
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 2,
		BaseDelay:  200 * time.Millisecond,
		MaxDelay:   2 * time.Second,
	}
}

// IsIdempotent 判断 HTTP 方法是否语义上幂等。
// GET/HEAD/PUT/DELETE/OPTIONS：幂等；POST/PATCH：非幂等。
// 注意：部分 POST 实现也可能幂等（按业务），但默认保守不重试，避免重复副作用。
func IsIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

// IsRetriableError 判断传输层错误是否可重试。
// 仅网络层瞬时故障（超时/不可达）可重试；响应体超限或已收到响应的业务错误不重试。
func IsRetriableError(err error) bool {
	if err == nil {
		return false
	}
	// ErrUpstreamTimeout / ErrServerUnreachable：网络抖动，可重试
	// ErrBodyTooLarge / ErrUpstreamError（业务层）：上游已应答或响应异常，重试无意义
	return errors.Is(err, ErrUpstreamTimeout) || errors.Is(err, ErrServerUnreachable)
}

// BackoffDelay 计算第 attempt 次重试前的等待时长（attempt 从 0 起）。
// delay = min(MaxDelay, BaseDelay * 2^attempt)。
// 注意：BaseDelay 与 MaxDelay 已由 SetRetry / DefaultRetryConfig 保证为正数。
func (c RetryConfig) BackoffDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	// 防止 attempt 过大导致溢出：超过 30 次直接取 MaxDelay
	if attempt > 30 {
		return c.MaxDelay
	}
	delay := c.BaseDelay << uint(attempt)
	if delay <= 0 || delay > c.MaxDelay {
		delay = c.MaxDelay
	}
	if delay < c.BaseDelay {
		delay = c.BaseDelay
	}
	return delay
}

// ShouldRetry 判断是否应继续重试。
//   - attempt < MaxRetries：还有重试预算
//   - 方法幂等：避免重放非幂等请求
//   - 错误可重试：避免重试业务错误
//   - 上下文未取消：超时预算已耗尽则停止
func (c RetryConfig) ShouldRetry(attempt int, method string, err error, ctxDone bool) bool {
	if attempt >= c.MaxRetries {
		return false
	}
	if !IsIdempotent(method) {
		return false
	}
	if !IsRetriableError(err) {
		return false
	}
	if ctxDone {
		return false
	}
	return true
}
