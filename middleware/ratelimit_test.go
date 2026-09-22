package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func setupLimiter(t *testing.T, limits map[string]Limit) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewRateLimiter(rdb, limits), mr
}

func runLimited(t *testing.T, limiter *RateLimiter, role string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("request_id", "req-rl")
		c.Set("agent_role", role)
		c.Set("user_id", userID)
		c.Next()
	})
	r.GET("/probe", limiter.Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
	return w
}

func TestRateLimitAllowsNormalTraffic(t *testing.T) {
	limiter, _ := setupLimiter(t, map[string]Limit{"agent": {Capacity: 5, Refill: 2}})
	for i := 0; i < 5; i++ {
		if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusOK {
			t.Fatalf("容量内第 %d 个请求被误拒: %d", i+1, w.Code)
		}
	}
}

func TestRateLimitBlocksOverThreshold(t *testing.T) {
	limiter, _ := setupLimiter(t, map[string]Limit{"agent": {Capacity: 3, Refill: 2}})
	for i := 0; i < 3; i++ {
		if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusOK {
			t.Fatalf("容量内第 %d 个请求被误拒: %d", i+1, w.Code)
		}
	}
	w := runLimited(t, limiter, "agent", 1)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("超阈值应 429，实际 %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "RATE_LIMITED" {
		t.Fatalf("429 响应 code = %v", body["code"])
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("429 响应缺少 Retry-After")
	}
}

func TestRateLimitRefillsOverTime(t *testing.T) {
	limiter, mr := setupLimiter(t, map[string]Limit{"agent": {Capacity: 1, Refill: 2}})
	if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusOK {
		t.Fatalf("首个请求应放行: %d", w.Code)
	}
	if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusTooManyRequests {
		t.Fatalf("桶空应 429: %d", w.Code)
	}
	// 时间推进 1 秒：Refill=2/s 应补充 2 个令牌
	mr.SetTime(time.Now().Add(1 * time.Second))
	if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusOK {
		t.Fatalf("时间推进后应恢复放行: %d", w.Code)
	}
}

func TestRateLimitIsolatedByUser(t *testing.T) {
	limiter, _ := setupLimiter(t, map[string]Limit{"agent": {Capacity: 1, Refill: 0.001}})
	// 用户 1 耗尽自己的桶
	_ = runLimited(t, limiter, "agent", 1)
	if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusTooManyRequests {
		t.Fatalf("用户1 桶空应 429: %d", w.Code)
	}
	// 用户 2 独立桶不受影响
	if w := runLimited(t, limiter, "agent", 2); w.Code != http.StatusOK {
		t.Fatalf("用户2 独立桶应放行: %d", w.Code)
	}
}

func TestRateLimitRoleQuotaDiffers(t *testing.T) {
	limiter, _ := setupLimiter(t, DefaultRoleLimits())
	// agent 120 容量：第 121 个应 429
	for i := 0; i < 120; i++ {
		if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusOK {
			t.Fatalf("agent 容量内第 %d 个被误拒: %d", i+1, w.Code)
		}
	}
	if w := runLimited(t, limiter, "agent", 1); w.Code != http.StatusTooManyRequests {
		t.Fatalf("agent 第 121 个应 429: %d", w.Code)
	}
	// admin 600 容量：同一用户无关，用 admin 用户验证更高配额可用
	if w := runLimited(t, limiter, "admin", 9); w.Code != http.StatusOK {
		t.Fatalf("admin 应放行: %d", w.Code)
	}
}

func TestRateLimitDegradedWhenRedisDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// 指向不可达地址，模拟 Redis 故障
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:59998", DialTimeout: 50 * time.Millisecond})
	t.Cleanup(func() { _ = rdb.Close() })
	limiter := NewRateLimiter(rdb, DefaultRoleLimits())
	for i := 0; i < 3; i++ {
		w := runLimited(t, limiter, "agent", 1)
		if w.Code != http.StatusOK {
			t.Fatalf("Redis 不可用应降级放行，第 %d 个请求返回 %d", i+1, w.Code)
		}
	}
}
