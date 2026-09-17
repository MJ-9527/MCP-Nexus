package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Limit 令牌桶参数：Capacity 桶容量（突发上限），Refill 每秒补充令牌数。
// 约定限额（个/分钟）→ Capacity = 分钟配额，Refill = 配额/60。
type Limit struct {
	Capacity int64
	Refill   float64
}

// DefaultRoleLimits 项目约定的角色限额：admin 600/min、developer 300/min、agent 120/min。
func DefaultRoleLimits() map[string]Limit {
	return map[string]Limit{
		"admin":     {Capacity: 600, Refill: 10},
		"developer": {Capacity: 300, Refill: 5},
		"agent":     {Capacity: 120, Refill: 2},
	}
}

// defaultLimit 未识别角色的兜底限额（60/min）。
var defaultLimit = Limit{Capacity: 60, Refill: 1}

// tokenBucketScript 令牌桶 Lua 脚本：检查+扣减原子执行，返回 1=放行 0=限流。
// 时间取 Redis 服务器 TIME，避免多实例网关时钟漂移。
var tokenBucketScript = redis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill = tonumber(ARGV[2])

local clock = redis.call('TIME')
local now_ms = clock[1] * 1000 + math.floor(clock[2] / 1000)

local bucket = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(bucket[1])
local ts = tonumber(bucket[2])
if tokens == nil then
	tokens = capacity
	ts = now_ms
end

local elapsed = now_ms - ts
if elapsed < 0 then elapsed = 0 end
tokens = math.min(capacity, tokens + elapsed / 1000.0 * refill)

local allowed = 0
if tokens >= 1 then
	tokens = tokens - 1
	allowed = 1
end
redis.call('HMSET', key, 'tokens', tokens, 'ts', now_ms)
redis.call('EXPIRE', key, math.ceil(capacity / refill) * 2 + 60)
return allowed
`)

// RateLimiter Redis 令牌桶限流器（B7）：Key 按角色+用户隔离，Redis 不可用时降级放行并记日志。
type RateLimiter struct {
	rdb    redis.UniversalClient
	limits map[string]Limit
	// checkTimeout 单次限流检查的超时，防止 Redis 慢查询拖垮网关
	checkTimeout time.Duration
}

func NewRateLimiter(rdb redis.UniversalClient, limits map[string]Limit) *RateLimiter {
	return &RateLimiter{rdb: rdb, limits: limits, checkTimeout: 100 * time.Millisecond}
}

// Middleware 返回限流中间件。身份取 JWT 注入的 agent_role/user_id（B5/B6），
// 未认证请求按 clientIP 隔离。超阈值返回 429 + Retry-After。
func (l *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := l.limitFor(c.GetString("agent_role"))
		key := l.keyFor(c)
		allowed, retryAfter, err := l.take(c.Request.Context(), key, limit)
		if err != nil {
			// Redis 不可用：降级放行（fail-open），记录日志人工介入
			log.Printf("[ratelimit] Redis 不可用，降级放行 key=%s err=%v", key, err)
			c.Next()
			return
		}
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit.Capacity))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":        "RATE_LIMITED",
				"message":     "请求过于频繁，请稍后重试",
				"retry_after": int(retryAfter.Seconds()) + 1,
				"request_id":  c.GetString("request_id"),
			})
			return
		}
		c.Next()
	}
}

func (l *RateLimiter) limitFor(role string) Limit {
	if limit, ok := l.limits[role]; ok {
		return limit
	}
	return defaultLimit
}

// keyFor 限流 Key：角色+用户（认证后），否则 clientIP（匿名兜底）。
func (l *RateLimiter) keyFor(c *gin.Context) string {
	if userID := c.GetInt64("user_id"); userID > 0 {
		return fmt.Sprintf("rl:%s:%d", c.GetString("agent_role"), userID)
	}
	return fmt.Sprintf("rl:ip:%s", c.ClientIP())
}

// take 原子扣减一个令牌。返回是否放行、限流时建议的重试等待时长。
func (l *RateLimiter) take(ctx context.Context, key string, limit Limit) (bool, time.Duration, error) {
	checkCtx, cancel := context.WithTimeout(ctx, l.checkTimeout)
	defer cancel()
	allowed, err := tokenBucketScript.Run(checkCtx, l.rdb, []string{key},
		limit.Capacity, limit.Refill).Int()
	if err != nil {
		return false, 0, err
	}
	if allowed == 1 {
		return true, 0, nil
	}
	// 桶空：补充一个令牌所需的时间即建议重试间隔
	return false, time.Duration(float64(time.Second) / limit.Refill), nil
}
