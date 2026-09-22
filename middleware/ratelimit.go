package middleware

import (
<<<<<<< HEAD
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	requests []time.Time
	limit    int
	window   time.Duration
}

type RateLimiter struct {
	mu            sync.RWMutex
	buckets       map[string]*rateBucket
	defaultLimit  int
	defaultWindow time.Duration
}

var defaultLimiter *RateLimiter

func init() {
	defaultLimiter = NewRateLimiter(60, 60*time.Second)
}

func NewRateLimiter(defaultLimit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:       make(map[string]*rateBucket),
		defaultLimit:  defaultLimit,
		defaultWindow: window,
	}
}

func uidKey(id int64) string {
	if id == 0 {
		return "anon"
	}
	return string(rune('a' + byte(id%26))) + string(rune('0' + byte(id%10)))
}

func (rl *RateLimiter) Allow(key string, limit int, window time.Duration) (bool, int, int, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok || window != b.window {
		b = &rateBucket{limit: limit, window: window}
		rl.buckets[key] = b
	}
	cutoff := now.Add(-window)
	j := 0
	for i := range b.requests {
		if b.requests[i].After(cutoff) {
			b.requests[j] = b.requests[i]
			j++
		}
	}
	b.requests = b.requests[:j]
	remaining := limit - len(b.requests)
	if len(b.requests) >= limit {
		retryAfter := int(b.requests[0].Add(window).Sub(now).Seconds()) + 1
		if retryAfter < 1 {
			retryAfter = 1
		}
		return false, retryAfter, limit, 0
	}
	b.requests = append(b.requests, now)
	remaining--
	return true, 0, limit, remaining
}

func statusJSON(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"code": code, "message": message, "request_id": c.GetString("request_id")})
}

func RateLimit(defaultLimit int, window time.Duration) gin.HandlerFunc {
	rl := NewRateLimiter(defaultLimit, window)
	return func(c *gin.Context) {
		userID, hasUser := GetCurrentUserID(c)
		k := "global"
		if hasUser {
			k = "u:" + uidKey(userID)
		}
		allowed, retryAfter, limit, remaining := rl.Allow(k, defaultLimit, window)
		c.Header("X-RateLimit-Limit", itoa(limit))
		c.Header("X-RateLimit-Remaining", itoa(remaining))
		if retryAfter > 0 {
			c.Header("Retry-After", itoa(retryAfter))
		}
		if !allowed {
			statusJSON(c, 429, "RATE_LIMITED", "rate limit exceeded")
			c.Abort()
=======
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

// DefaultRoleLimits 项目约定的角色限额：platform_admin 600/min、tool_developer 300/min、agent_caller 120/min。
func DefaultRoleLimits() map[string]Limit {
	return map[string]Limit{
		RoleAdmin:     {Capacity: 600, Refill: 10},
		RoleDeveloper: {Capacity: 300, Refill: 5},
		RoleAgent:     {Capacity: 120, Refill: 2},
		// 兼容合并前签发的旧角色值；新代码统一使用上述三个规范名。
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
>>>>>>> origin/pull-request
			return
		}
		c.Next()
	}
}

<<<<<<< HEAD
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
=======
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
>>>>>>> origin/pull-request
