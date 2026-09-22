package middleware

import (
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
			return
		}
		c.Next()
	}
}

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